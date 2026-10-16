// Package saga runs sequential steps with reverse compensation planned,
// not executed.
//
// Sequential only: no parallel branches, no retries, no durable log.
// On step failure Run returns the failure immediately along with the
// unwind plan (completed step names in order); the caller queues that
// plan for a worker. Run itself never compensates.
package saga

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Step is one unit of work. Name is a stable identifier (used in results,
// logs and spans). Compensate undoes Do and may be nil when there is
// nothing to undo.
type Step[T any] interface {
	Name() string
	Do(context.Context, *T) error
	Compensate(context.Context, *T) error
}

// Saga is a named sequential step list. Build it with New, which
// guarantees a uuid ID plus non-nil logger and tracer (customizable via
// options). ID, Name and Steps stay exported; logger and tracer are set
// via options. Literals bypassing New still run: Run falls back to the
// same defaults.
type Saga[T any] struct {
	ID     string
	Name   string
	Steps  []Step[T]
	logger *slog.Logger
	tracer trace.Tracer
}

// DefaultTracerName is the instrumentation scope for sagas that do
// not customize it via WithTracerName.
const DefaultTracerName = "pkg/saga"

// Option configures a Saga. The zero option set keeps the defaults.
type Option[T any] func(*Saga[T])

// WithLogger sets the saga logger. Nil keeps slog.Default().
func WithLogger[T any](l *slog.Logger) Option[T] {
	return func(s *Saga[T]) {
		if l != nil {
			s.logger = l
		}
	}
}

// WithTracerName resolves otel.Tracer(name) and sets it. Empty keeps
// the DefaultTracerName tracer.
func WithTracerName[T any](name string) Option[T] {
	return func(s *Saga[T]) {
		if name != "" {
			s.tracer = otel.Tracer(name)
		}
	}
}

// New builds a Saga with a uuid ID and guaranteed log/tracing:
// logger defaults to slog.Default(), tracer to
// otel.Tracer(DefaultTracerName). Opts override either; nil/empty
// options keep the defaults, so both fields are non-nil on return.
func New[T any](name string, steps []Step[T], opts ...Option[T]) Saga[T] {
	s := Saga[T]{ID: uuid.NewString(), Name: name, Steps: steps}
	for _, opt := range opts {
		opt(&s)
	}

	if s.logger == nil {
		s.logger = slog.Default()
	}

	if s.tracer == nil {
		s.tracer = otel.Tracer(DefaultTracerName)
	}

	s.logger = s.logger.With(slog.String("saga_id", s.ID), slog.String("saga_name", s.Name))

	return s
}

// Run executes steps in order, stopping at the first failure (or cancelled
// ctx). On ordinary failure Run returns the failure immediately with the
// unwind plan (completed step names in order) for the caller to queue;
// Run itself never compensates. Returns nil on success.
func (s Saga[T]) Run(ctx context.Context, item *T) error {
	ctx, span := s.tracer.Start(ctx, "saga."+s.Name,
		trace.WithAttributes(attribute.String("saga.id", s.ID), attribute.String("saga.name", s.Name)))
	defer span.End()

	var completed []string
	for _, st := range s.Steps {
		// timeout: if the caller ctx is cancelled, we stop and return the
		// plan for the steps completed so far. The failure returns immediately.
		if ctx.Err() != nil {
			span.RecordError(ctx.Err())
			span.SetStatus(codes.Error, "context cancelled")
			return &RunError{
				SagaID: s.ID, SagaName: s.Name,
				FailedStep: st.Name(), StepErr: ctx.Err(),
				Completed: append([]string(nil), completed...),
			}
		}

		if doErr := s.executeStepDo(ctx, st, item); doErr != nil {
			var indoubt *InDoubtError
			if errors.As(doErr, &indoubt) {
				s.logger.WarnContext(ctx, "saga step in-doubt, skipping unwind",
					slog.String("step", st.Name()), slog.String("error", doErr.Error()))
				span.RecordError(doErr)
				span.SetStatus(codes.Error, "step in-doubt")
				return &RunError{
					SagaID: s.ID, SagaName: s.Name,
					FailedStep: st.Name(), StepErr: doErr,
					Completed: append([]string(nil), completed...),
					InDoubt:   true,
				}
			}

			s.logger.ErrorContext(ctx, "saga step failed",
				slog.String("step", st.Name()), slog.String("error", doErr.Error()))
			span.RecordError(doErr)
			span.SetStatus(codes.Error, "step failed")

			return &RunError{
				SagaID: s.ID, SagaName: s.Name,
				FailedStep: st.Name(), StepErr: doErr,
				Completed: append([]string(nil), completed...),
			}
		}

		completed = append(completed, st.Name())
	}

	return nil
}

func (s Saga[T]) executeStepDo(ctx context.Context, st Step[T], item *T) error {
	ctx, span := s.tracer.Start(ctx, "saga.step."+st.Name(),
		trace.WithAttributes(attribute.String("step", st.Name())))
	defer span.End()

	if err := st.Do(ctx, item); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "step failed")
		return err
	}

	return nil
}
