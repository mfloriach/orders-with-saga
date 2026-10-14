package steps

import (
	"context"
	"fmt"
	"paymentgateway/internal/payment/models"
	"paymentgateway/pkg/saga"
)

type PendingPayment struct {
	Payments models.PaymentRepository
}

func (s PendingPayment) Run(p *models.Payment) saga.Step {
	return saga.Step{
		Name: "insert-payment",
		Do: func(ctx context.Context) error {
			return s.Payments.InsertPending(ctx, p)
		},
		Compensate: func(c context.Context) error {
			if err := s.Payments.MarkFailed(c, p); err != nil {
				return fmt.Errorf("payment: failed to mark failed: %w", err)
			}

			return nil
		},
	}
}
