package app

import (
	"context"
	"encoding/json"
	"fmt"

	"paymentgateway/internal/webhooks"
)

// WebhookPublisher is the AMQP webhooks.Publisher: one persistent message
// per event on the payments.events exchange.
type WebhookPublisher struct {
	broker *Broker
}

func NewWebhookPublisher(b *Broker) webhooks.Publisher { return &WebhookPublisher{broker: b} }

func (p *WebhookPublisher) Publish(ctx context.Context, e webhooks.Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal webhook event: %w", err)
	}
	if err := p.broker.Publish(ctx, WebhookExchange, WebhookRoute, body, nil); err != nil {
		return fmt.Errorf("publish webhook event %s: %w", e.PaymentID, err)
	}
	return nil
}
