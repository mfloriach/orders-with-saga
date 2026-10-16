// Package saga runs sequential steps with reverse compensation.
//
// Sequential only: no parallel branches, no retries, no durable log.
// On step failure Run returns immediately and completed steps are
// compensated in reverse order in the background, best-effort:
// compensation errors are logged and unwinding continues.
package saga

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DefaultCompensateTimeout bounds compensations detached from a cancelled caller ctx.
const DefaultCompensateTimeout = 5 * time.Second

// Step is one unit of work. Name is a stable identifier (used in results,
// logs and spans). Compensate undoes Do and may be nil when there is
// nothing to undo.
type Step struct {
	Name       string
	Do         func(ctx context.Context) error
	Compensate func(ctx context.Context) error
}

// Saga is a named sequential step list. ID defaults to a uuid at Run.
// Logger defaults to slog.Default, TracerName to "pkg/saga".
// CompensateTimeout bounds compensations when the caller ctx is cancelled.
// OnStep observes every executed step synchronously with its result; it
// returns nothing and cannot fail the saga.
type Saga struct {
	ID                string
	Name              string
	Steps             []Step
	Logger            *slog.Logger
	TracerName        string
	CompensateTimeout time.Duration
	OnStep            func(ctx context.Context, sagaID, step string, err error)
}

func (s Saga) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s Saga) tracer() trace.Tracer {
	name := s.TracerName
	if name == "" {
		name = "pkg/saga"
	}
	return otel.Tracer(name)
}

func (s Saga) compensateTimeout() time.Duration {
	if s.CompensateTimeout > 0 {
		return s.CompensateTimeout
	}
	return DefaultCompensateTimeout
}

// Run executes steps in order, stopping at the first failure (or cancelled
// ctx). On ordinary failure Run returns the failure immediately and the
// completed steps are compensated in reverse in the background. Returns nil
// on success.
func (s Saga) Run(ctx context.Context) error {
	id := s.ID
	if id == "" {
		id = uuid.NewString()
	}
	log := s.logger().With(slog.String("saga_id", id), slog.String("saga_name", s.Name))
	tr := s.tracer()
	ctx, span := tr.Start(ctx, "saga."+s.Name,
		trace.WithAttributes(attribute.String("saga.id", id), attribute.String("saga.name", s.Name)))
	defer span.End()

	var completed []string
	// ponytail: O(n^2) completed-name lookup replaced by index loop; steps are few.
	for _, st := range s.Steps {
		// timeout: if the caller ctx is cancelled, we stop and unwind in the
		// background. The failure returns immediately.
		if ctx.Err() != nil {
			span.RecordError(ctx.Err())
			span.SetStatus(codes.Error, "context cancelled")
			return s.unwindAsync(ctx, log, tr, id, completed, st.Name, ctx.Err())
		}

		doErr := s.do(ctx, tr, st)
		if s.OnStep != nil {
			s.OnStep(ctx, id, st.Name, doErr)
		}
		if doErr != nil {
			var indoubt *InDoubtError
			if errors.As(doErr, &indoubt) {
				log.WarnContext(ctx, "saga step in-doubt, skipping unwind",
					slog.String("step", st.Name), slog.String("error", doErr.Error()))
				span.RecordError(doErr)
				span.SetStatus(codes.Error, "step in-doubt")
				return &RunError{
					SagaID: id, SagaName: s.Name,
					FailedStep: st.Name, StepErr: doErr,
					Completed: append([]string(nil), completed...),
					InDoubt:   true,
				}
			}
			log.ErrorContext(ctx, "saga step failed",
				slog.String("step", st.Name), slog.String("error", doErr.Error()))
			span.RecordError(doErr)
			span.SetStatus(codes.Error, "step failed")
			return s.unwindAsync(ctx, log, tr, id, completed, st.Name, doErr)
		}

		completed = append(completed, st.Name)
	}
	return nil
}

func (s Saga) do(ctx context.Context, tr trace.Tracer, st Step) error {
	ctx, span := tr.Start(ctx, "saga.step."+st.Name,
		trace.WithAttributes(attribute.String("step", st.Name)))
	defer span.End()
	if st.Do == nil {
		return nil
	}
	if err := st.Do(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "step failed")
		return err
	}
	return nil
}

// unwindAsync builds the failure result and compensates completed steps in
// reverse in the background, then returns immediately. The unwind runs on a
// context detached from the caller (so a disconnected client never aborts
// it) bounded by the compensate timeout, with spans re-rooted under a fresh
// trace since the request span may close on return. Compensation errors are
// logged with saga identity; unwinding continues past them.
func (s Saga) unwindAsync(ctx context.Context, log *slog.Logger, tr trace.Tracer, id string, completed []string, failed string, stepErr error) *RunError {
	out := &RunError{
		SagaID: id, SagaName: s.Name,
		FailedStep: failed, StepErr: stepErr,
		Completed: append([]string(nil), completed...),
	}
	byName := make(map[string]Step, len(s.Steps))
	for _, st := range s.Steps {
		byName[st.Name] = st
	}
	go func() {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.compensateTimeout())
		defer cancel()
		uctx, uspan := tr.Start(cctx, "saga."+s.Name+".unwind",
			trace.WithAttributes(attribute.String("saga.id", id), attribute.String("saga.name", s.Name)))
		defer uspan.End()
		for i := len(completed) - 1; i >= 0; i-- {
			name := completed[i]
			st := byName[name]
			if st.Compensate == nil {
				continue
			}
			cctx, cspan := tr.Start(uctx, "saga.compensate."+name,
				trace.WithAttributes(attribute.String("step", name)))
			err := st.Compensate(cctx)
			if err != nil {
				cspan.RecordError(err)
				cspan.SetStatus(codes.Error, "compensate failed")
				log.WarnContext(cctx, "saga compensate failed",
					slog.String("step", name), slog.String("error", err.Error()))
			}
			cspan.End()
		}
	}()
	return out
}
