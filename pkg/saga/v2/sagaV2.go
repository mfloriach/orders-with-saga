package v2

import (
	"context"
	"errors"
	"fmt"
	"paymentgateway/internal/domain"
	"time"
)

type PaymentWorkflow struct {
	domain.Payment

	Pending       *domain.PaymentPending
	Authenticated *domain.PaymentAuthenticated
	Captured      *domain.PaymentCaptured
}

type Step interface {
	Name() string
	Do(context.Context, *PaymentWorkflow) error
	Compensate(context.Context, *PaymentWorkflow) error
}

type Saga struct {
	steps               []Step
	compensationTimeout time.Duration
}

func New(steps ...Step) *Saga {
	return &Saga{
		steps:               steps,
		compensationTimeout: 10 * time.Second,
	}
}

func (s *Saga) Execute(
	ctx context.Context,
	w *PaymentWorkflow,
) error {
	completed := make([]Step, 0, len(s.steps))

	for _, step := range s.steps {
		if err := ctx.Err(); err != nil {
			return s.rollback(ctx, w, completed, err)
		}

		if err := step.Do(ctx, w); err != nil {
			cause := fmt.Errorf(
				"saga step %q failed: %w",
				step.Name(),
				err,
			)

			return s.rollback(ctx, w, completed, cause)
		}

		completed = append(completed, step)
	}

	return nil
}

func (s *Saga) rollback(
	ctx context.Context,
	w *PaymentWorkflow,
	completed []Step,
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
