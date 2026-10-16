package steps

import (
	"context"
	"fmt"
	"paymentgateway/internal/domain"
	"paymentgateway/pkg/saga"
)

type PendingPayment struct {
	Payments domain.PaymentRepository
}

func (s PendingPayment) Run(p *domain.Payment) saga.Step {
	return saga.Step{
		Name: "insert-payment",
		Do: func(ctx context.Context) error {
			return s.Payments.InsertPending(ctx, p)
		},
		Compensate: func(ctx context.Context) error {
			if err := s.Payments.MarkFailed(ctx, p); err != nil {
				return fmt.Errorf("payment: failed to mark failed: %w", err)
			}

			return nil
		},
	}
}
