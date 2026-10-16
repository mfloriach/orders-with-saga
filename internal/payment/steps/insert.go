package steps

import (
	"context"
	"paymentgateway/internal/domain"
)

type PendingPayment struct {
	Payments domain.PaymentRepository
}

func (s PendingPayment) Name() string { return "insert-payment" }

func (s PendingPayment) Do(ctx context.Context, p *domain.Payment) error {
	return s.Payments.InsertPending(ctx, p)
}

func (s PendingPayment) Compensate(ctx context.Context, p *domain.Payment) error {
	return s.Payments.MarkFailed(ctx, p)
}
