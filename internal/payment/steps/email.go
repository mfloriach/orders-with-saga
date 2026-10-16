package steps

import (
	"context"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/webhooks"
)

// Email publishes the customer notification event. It is deliberately not a
// saga.Step: email goes out after the saga succeeds, so a notification
// failure must never unwind a captured payment.
type Email struct {
	Publisher webhooks.EmailPublisher
}

func (s Email) Publish(ctx context.Context, p *domain.Payment) error {
	return s.Publisher.Publish(ctx, webhooks.Event{
		PaymentID: p.ID, MerchantID: p.MerchantID, Amount: p.Amount, Status: string(p.Status),
	})
}
