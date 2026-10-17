package v2

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type SagaError struct {
	Step    string
	Err     error
	Details string
}

func (s SagaError) Error() string {
	return fmt.Sprintf("saga error: %v, %v, %v", s.Step, s.Err, s.Details)
}

type Step[T any] interface {
	Name() string
	Do(context.Context, *T) error
	Compensate(context.Context, *T) error
}

type Saga[T any] struct {
	steps               []Step[T]
	compensationTimeout time.Duration
}

func New[T any](steps ...Step[T]) *Saga[T] {
	return &Saga[T]{
		steps:               steps,
		compensationTimeout: 10 * time.Second,
	}
}

func (s *Saga[T]) Execute(ctx context.Context, w *T) error {
	completed := make([]Step[T], 0, len(s.steps))

	for _, step := range s.steps {
		if err := ctx.Err(); err != nil {
			return SagaError{
				Step:    step.Name(),
				Err:     ctx.Err(),
				Details: "timeout",
			}
		}

		if err := step.Do(ctx, w); err != nil {
			return SagaError{
				Step:    step.Name(),
				Err:     err,
				Details: "step failed",
			}
		}

		completed = append(completed, step)
	}

	return nil
}

func (s *Saga[T]) Reconsilate(
	ctx context.Context,
	w *T,
	completed []Step[T],
	cause error,
) error {
	// Compensation must still run if the original context was canceled.
	ctx = context.WithoutCancel(ctx)

	for i := len(completed) - 1; i >= 0; i-- {
		step := completed[i]

		timeout := s.compensationTimeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}

		compCtx, cancel := context.WithTimeout(ctx, timeout)
		err := step.Compensate(compCtx, w)
		cancel()

		if err != nil {
			cause = errors.Join(
				cause,
				fmt.Errorf(
					"compensating step %q failed: %w",
					step.Name(),
					err,
				),
			)
		}
	}

	return cause
}
