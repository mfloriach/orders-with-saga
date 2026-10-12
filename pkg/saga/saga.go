// Package saga runs sequential steps with reverse compensation.
//
// Sequential only: no parallel branches, no retries, no durable log.
// On step failure completed steps are compensated in reverse order,
// best-effort: compensation errors are collected and unwinding continues.
package saga

import (
	"context"
	"fmt"
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
type Saga struct {
	ID                string
	Name              string
	Steps             []Step
	Logger            *slog.Logger
	TracerName        string
	CompensateTimeout time.Duration
}

// StepError pairs a step name with its compensation error.
type StepError struct {
	Step string
	Err  error
}

func (e StepError) Error() string { return fmt.Sprintf("compensate %s: %v", e.Step, e.Err) }
func (e StepError) Unwrap() error { return e.Err }

// RunError is the failure result. Completed holds executed step names in
// order — the resume set a future reconciler persists. CompensateErrs holds
// per-step undo failures, if any.
type RunError struct {
	SagaID         string
	SagaName       string
	FailedStep     string
	StepErr        error
	Completed      []string
	CompensateErrs []StepError
}

func (e *RunError) Error() string {
	return fmt.Sprintf("saga %s: step %s failed: %v (%d compensate errors)",
		e.SagaID, e.FailedStep, e.StepErr, len(e.CompensateErrs))
}

// Unwrap returns the step error for errors.Is/As.
func (e *RunError) Unwrap() error { return e.StepErr }

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
// ctx) and compensating completed steps in reverse. Returns nil on success.
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
		// timeout: if the caller ctx is cancelled, we stop and unwind. Compensations are still run, but bounded.
		if ctx.Err() != nil {
			span.RecordError(ctx.Err())
			span.SetStatus(codes.Error, "context cancelled")
			return s.unwind(ctx, log, tr, id, completed, st.Name, ctx.Err())
		}

		if err := s.do(ctx, tr, st); err != nil {
			log.ErrorContext(ctx, "saga step failed",
				slog.String("step", st.Name), slog.String("error", err.Error()))
			span.RecordError(err)
			span.SetStatus(codes.Error, "step failed")
			return s.unwind(ctx, log, tr, id, completed, st.Name, err)
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

func (s Saga) unwind(ctx context.Context, log *slog.Logger, tr trace.Tracer, id string, completed []string, failed string, stepErr error) *RunError {
	out := &RunError{
		SagaID: id, SagaName: s.Name,
		FailedStep: failed, StepErr: stepErr,
		Completed: append([]string(nil), completed...),
	}
	// Compensations must still run after client disconnect, but bounded.
	cctx := ctx
	cancel := context.CancelFunc(func() {})
	if ctx.Err() != nil {
		cctx, cancel = context.WithTimeout(context.Background(), s.compensateTimeout())
		defer cancel()
	}
	byName := make(map[string]Step, len(s.Steps))
	for _, st := range s.Steps {
		byName[st.Name] = st
	}
	for i := len(completed) - 1; i >= 0; i-- {
		name := completed[i]
		st := byName[name]
		if st.Compensate == nil {
			continue
		}
		cctx, cspan := tr.Start(cctx, "saga.compensate."+name,
			trace.WithAttributes(attribute.String("step", name)))
		err := st.Compensate(cctx)
		if err != nil {
			cspan.RecordError(err)
			cspan.SetStatus(codes.Error, "compensate failed")
			log.WarnContext(cctx, "saga compensate failed",
				slog.String("step", name), slog.String("error", err.Error()))
			out.CompensateErrs = append(out.CompensateErrs, StepError{Step: name, Err: err})
		}
		cspan.End()
	}
	cancel()
	return out
}
