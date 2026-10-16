package steps

import (
	"context"
	"fmt"
	"log/slog"
	"paymentgateway/internal/domain"
	"paymentgateway/internal/webhooks"
)

type Webhook struct {
	Publisher webhooks.Publisher
}

func (s Webhook) Name() string { return "webhook" }

func (s Webhook) Do(ctx context.Context, p *domain.Payment) error {
	evt := webhooks.Event{
		PaymentID:  p.ID,
		MerchantID: p.MerchantID,
		Amount:     p.Amount,
		Status:     string(p.Status),
	}
	if err := s.Publisher.Publish(ctx, evt); err != nil {
		slog.ErrorContext(ctx, "webhook publish failed",
			slog.String("payment_id", p.ID), slog.String("error", err.Error()))
		return fmt.Errorf("payment: failed to publish webhook event %s: %w", p.ID, err)
	}
	return nil
}

func (s Webhook) Compensate(ctx context.Context, p *domain.Payment) error {
	return nil
}
