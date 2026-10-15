// Package webhooks carries the async merchant-notification contract:
// the payment service publishes Events, the worker delivers them.
// It depends on nothing broker-specific; the AMQP implementation lives
// in internal/app so this package stays importable from the service.
package webhooks

import (
	"context"
	"sync"
)

// Event is the webhook payload for one completed payment.
type Event struct {
	PaymentID  string `json:"payment_id"`
	MerchantID string `json:"merchant_id"`
	Amount     int64  `json:"amount"`
	Status     string `json:"status"`
}

// Webhook topology. The worker consumes DeliverQueue; failed deliveries
// past the attempt cap are parked in DLQueue via the dead-letter exchange.
// Shared with internal/app, which declares it at boot.
const (
	Exchange     = "payments.events"
	DLX          = "payments.dlx"
	DeliverQueue = "webhook.deliver"
	DLQueue      = "webhook.dlq"
	RouteKey     = "webhook.created"
)

// Publisher delivers events to the broker. Implementations must be safe
// for concurrent use.
type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// FakePublisher records events for tests. Err, when non-nil, is returned
// from every Publish call.
type FakePublisher struct {
	mu     sync.Mutex
	Events []Event
	Err    error
}

func (f *FakePublisher) Publish(_ context.Context, e Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.Events = append(f.Events, e)
	return nil
}
