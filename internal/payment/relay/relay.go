package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"paymentgateway/internal/domain"
)

// Broker is the publisher seam the worker talks to. *config.Broker satisfies
// it; tests use a fake.
type Broker interface {
	Publish(ctx context.Context, routingKey string, body []byte) error
}

// CapturedEvent is the generic captured-payment event published to the
// broker. Delivery consumers bind to it in later slices.
type CapturedEvent struct {
	PaymentID  string `json:"payment_id"`
	MerchantID string `json:"merchant_id"`
	Status     string `json:"status"`
}

// CapturedEventRoutingKey routes generic captured-payment events on the
// payments.events topic exchange.
const CapturedEventRoutingKey = "payment.captured"

// Worker relays captured payments from the durable outbox to the message
// broker. It polls pending outbox rows on an interval and publishes one
// generic captured-payment event per row, marking the row processed on
// success and retrying failed publishes with bounded attempts.
type Worker struct {
	Outbox      domain.OutBoxPaymentRepository
	Publisher   Broker
	MaxAttempts int
	Interval    time.Duration
	Logger      *slog.Logger
}

func NewWorker(outbox domain.OutBoxPaymentRepository, p Broker, maxAttempts int, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = time.Second
	}
	return &Worker{
		Outbox:      outbox,
		Publisher:   p,
		MaxAttempts: maxAttempts,
		Interval:    interval,
		Logger:      slog.Default(),
	}
}

// Run polls the outbox until the context is cancelled. Unblocks on ctx.Done
// between polls; in-flight processing finishes within a poll.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		if err := w.poll(ctx); err != nil {
			w.Logger.ErrorContext(ctx, "outbox relay poll failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) poll(ctx context.Context) error {
	rows, err := w.Outbox.GetByStatus(ctx)
	if err != nil {
		return fmt.Errorf("get pending outbox payments: %w", err)
	}
	for _, row := range rows {
		w.process(ctx, row)
	}
	return nil
}

func (w *Worker) process(ctx context.Context, row *domain.OutboxPayment) {
	event := CapturedEvent{
		PaymentID:  row.PaymentID,
		MerchantID: row.MerchantID,
		Status:     string(row.Status),
	}
	body, err := json.Marshal(event)
	if err != nil {
		w.Logger.ErrorContext(ctx, "outbox relay event marshal failed", "payment_id", row.PaymentID, "error", err)
		return
	}

	if err := w.Publisher.Publish(ctx, CapturedEventRoutingKey, body); err != nil {
		w.Logger.ErrorContext(ctx, "outbox relay publish failed", "payment_id", row.PaymentID, "error", err)
		if incErr := w.Outbox.IncrementAttempts(ctx, row.ID, err.Error()); incErr != nil {
			w.Logger.ErrorContext(ctx, "outbox relay increment attempts failed", "payment_id", row.PaymentID, "error", incErr)
			return
		}
		if int(row.Attempts)+1 >= w.MaxAttempts {
			if failErr := w.Outbox.MarkFailed(ctx, row.ID, err.Error()); failErr != nil {
				w.Logger.ErrorContext(ctx, "outbox relay mark failed failed", "payment_id", row.PaymentID, "error", failErr)
			}
		}
		return
	}

	if err := w.Outbox.MarkProcessed(ctx, row.ID); err != nil {
		w.Logger.ErrorContext(ctx, "outbox relay mark processed failed", "payment_id", row.PaymentID, "error", err)
	}
}
