package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"paymentgateway/internal/merchant"
	"paymentgateway/pkg/apiclient"

	amqp "github.com/rabbitmq/amqp091-go"
)

// attemptsHeader counts delivery tries across requeues. Missing means first try.
const attemptsHeader = "x-attempts"

// httpTimeout bounds one merchant POST.
const httpTimeout = 5 * time.Second

// MerchantLookup resolves the merchant for an event. *merchant.Repository
// satisfies it.
type MerchantLookup interface {
	FindByID(ctx context.Context, id string) (*merchant.Merchant, error)
}

// Consumer delivers webhook events synchronously, one in-flight message at
// a time. Shutdown via ctx finishes the current delivery, then returns;
// unacked work is requeued by the broker.
type Consumer struct {
	ch          *amqp.Channel
	merchants   MerchantLookup
	client      *apiclient.Client
	log         *slog.Logger
	maxAttempts int
	backoff     time.Duration
}

func NewConsumer(ch *amqp.Channel, merchants MerchantLookup, maxAttempts int, backoff time.Duration) *Consumer {
	return &Consumer{
		ch: ch, merchants: merchants,
		client:      apiclient.New(nil),
		log:         slog.Default(),
		maxAttempts: maxAttempts,
		backoff:     backoff,
	}
}

// Run consumes until ctx is cancelled or the channel closes.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("webhook consumer: qos: %w", err)
	}

	deliveries, err := c.ch.Consume(DeliverQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("webhook consumer: consume: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			_ = c.ch.Cancel("", false)
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return nil
			}
			c.handle(ctx, d)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, d amqp.Delivery) {
	var e Event
	if err := json.Unmarshal(d.Body, &e); err != nil {
		c.log.WarnContext(ctx, "webhook event unparseable, dead-lettering",
			slog.String("error", err.Error()))
		_ = d.Nack(false, false) // requeue=false routes to the DLQ via the DLX
		return
	}
	log := c.log.With(slog.String("payment_id", e.PaymentID), slog.String("merchant_id", e.MerchantID))

	m, err := c.merchants.FindByID(ctx, e.MerchantID)
	if err != nil {
		if err == merchant.ErrNotFound {
			log.WarnContext(ctx, "webhook merchant gone, dead-lettering")
			_ = d.Nack(false, false)
			return
		}
		log.WarnContext(ctx, "webhook merchant lookup failed, requeueing",
			slog.String("error", err.Error()))
		c.retryOrDeadLetter(ctx, log, d, e, err)
		return
	}
	if m.WebhookURL == nil || *m.WebhookURL == "" {
		log.InfoContext(ctx, "webhook merchant has no url, skipping")
		_ = d.Ack(false)
		return
	}

	// Single attempt per outer try: the consumer owns retry-vs-DLQ via the
	// x-attempts header, so inner retries would bypass the cap.
	resp, err := c.client.Do(ctx, func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, *m.WebhookURL, bytes.NewReader(d.Body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, apiclient.Policy{MaxAttempts: 1, Timeout: httpTimeout})
	if err != nil {
		log.WarnContext(ctx, "webhook delivery failed, requeueing",
			slog.String("error", err.Error()))
		c.retryOrDeadLetter(ctx, log, d, e, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		derr := fmt.Errorf("merchant returned %s", resp.Status)
		log.WarnContext(ctx, "webhook delivery failed, requeueing",
			slog.String("error", derr.Error()))
		c.retryOrDeadLetter(ctx, log, d, e, derr)
		return
	}
	_ = d.Ack(false)
}

// retryOrDeadLetter republishes the message with an incremented attempt
// count after backoff, or dead-letters it past the cap. Shutdown mid-backoff
// requeues the original, so no event is lost.
func (c *Consumer) retryOrDeadLetter(ctx context.Context, log *slog.Logger, d amqp.Delivery, e Event, cause error) {
	attempts := headerAttempts(d.Headers) + 1
	if attempts >= c.maxAttempts {
		log.WarnContext(ctx, "webhook attempts exhausted, dead-lettering",
			slog.Int("attempts", attempts), slog.String("error", cause.Error()))
		_ = d.Nack(false, false)
		return
	}
	select {
	case <-ctx.Done():
		_ = d.Nack(false, true)
		return
	case <-time.After(c.backoff * time.Duration(attempts)):
	}
	headers := amqp.Table{attemptsHeader: int32(attempts)}
	if err := c.publish(ctx, d.Body, headers); err != nil {
		log.WarnContext(ctx, "webhook requeue failed, requeueing original",
			slog.String("error", err.Error()))
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}

func (c *Consumer) publish(ctx context.Context, body []byte, headers amqp.Table) error {
	return c.ch.PublishWithContext(ctx, Exchange, RouteKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
		Body:         body,
	})
}

func headerAttempts(h amqp.Table) int {
	if h == nil {
		return 0
	}
	switch v := h[attemptsHeader].(type) {
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}
