package app

import (
	"context"
	"encoding/json"
	"fmt"

	"paymentgateway/internal/config"
	"paymentgateway/internal/webhooks"
)

// EmailPublisher is the AMQP webhooks.EmailPublisher: one persistent message
// per event on the payments.events exchange under the email route.
type EmailPublisher struct {
	broker *config.Broker
}

func NewEmailPublisher(b *config.Broker) webhooks.EmailPublisher { return &EmailPublisher{broker: b} }

func (p *EmailPublisher) Publish(ctx context.Context, e webhooks.Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal email event: %w", err)
	}
	if err := p.broker.Publish(ctx, config.WebhookExchange, webhooks.EmailRoute, body, nil); err != nil {
		return fmt.Errorf("publish email event %s: %w", e.PaymentID, err)
	}
	return nil
}
