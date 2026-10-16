package steps

import (
	"context"
	"fmt"
	"log/slog"
	"paymentgateway/internal/domain"
	"paymentgateway/internal/webhooks"
	"paymentgateway/pkg/saga"
)

type Webhook struct {
	Publisher webhooks.Publisher
}

func (s Webhook) Run(p *domain.Payment) saga.Step {
	return saga.Step{
		Name: "send-webhook",
		Do: func(ctx context.Context) error {
			evt := webhooks.Event{PaymentID: p.ID, MerchantID: p.MerchantID, Amount: p.Amount, Status: string(p.Status)}
			if err := s.Publisher.Publish(ctx, evt); err != nil {
				slog.ErrorContext(ctx, "webhook publish failed",
					slog.String("payment_id", p.ID), slog.String("error", err.Error()))
				return fmt.Errorf("payment: failed to publish webhook event %s: %w", p.ID, err)
			}
			return nil
		},
		Compensate: func(c context.Context) error {
			return nil
		},
	}
}
