package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"paymentgateway/internal/domain"
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
	FindByID(ctx context.Context, id string) (*domain.Merchant, error)
}

// EmailSender delivers one rendered email. The production implementation
// logs it (transport is a future SMTP configuration); tests use FakeSender.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// LogEmailSender is the EmailSender until SMTP exists: it records the
// rendered email in logs, which keeps delivery observable end to end.
type LogEmailSender struct{ log *slog.Logger }

func NewLogEmailSender(log *slog.Logger) *LogEmailSender {
	if log == nil {
		log = slog.Default()
	}
	return &LogEmailSender{log: log}
}

func (s *LogEmailSender) Send(ctx context.Context, to, subject, body string) error {
	s.log.InfoContext(ctx, "email sent", slog.String("to", to), slog.String("subject", subject), slog.String("body", body))
	return nil
}

// FakeSender records sent emails for tests. Err, when non-nil, is returned
// from every Send call.
type FakeSender struct {
	mu    sync.Mutex
	Sent  []SentEmail
	Calls int
	Err   error
}

// SentEmail is one recorded delivery.
type SentEmail struct {
	To      string
	Subject string
	Body    string
}

func (f *FakeSender) Send(_ context.Context, to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls++
	if f.Err != nil {
		return f.Err
	}
	f.Sent = append(f.Sent, SentEmail{To: to, Subject: subject, Body: body})
	return nil
}

func (f *FakeSender) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Calls
}

func (f *FakeSender) sent() []SentEmail {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]SentEmail(nil), f.Sent...)
}

// notifier delivers one event body to a merchant target. Webhook POSTs to a
// URL; email sends via an EmailSender.
type notifier interface {
	kind() string
	// target returns "" when there is nowhere to deliver (ack-skip).
	target(m *domain.Merchant) string
	send(ctx context.Context, target string, body []byte) error
}

type webhookNotifier struct{ client *apiclient.Client }

func (webhookNotifier) kind() string { return "webhook" }

func (webhookNotifier) target(m *domain.Merchant) string {
	if m.WebhookURL == nil || *m.WebhookURL == "" {
		return ""
	}
	return *m.WebhookURL
}

func (n webhookNotifier) send(ctx context.Context, target string, body []byte) error {
	// Single attempt per outer try: the consumer owns retry-vs-DLQ via the
	// x-attempts header, so inner retries would bypass the cap.
	resp, err := n.client.Do(ctx, func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, apiclient.Policy{MaxAttempts: 1, Timeout: httpTimeout})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("merchant returned %s", resp.Status)
	}
	return nil
}

type emailNotifier struct{ sender EmailSender }

func (emailNotifier) kind() string { return "email" }

func (emailNotifier) target(m *domain.Merchant) string { return m.Email }

func (n emailNotifier) send(ctx context.Context, target string, body []byte) error {
	var e Event
	if err := json.Unmarshal(body, &e); err != nil {
		return fmt.Errorf("email event unparseable: %w", err)
	}
	return n.sender.Send(ctx, target,
		fmt.Sprintf("Payment %s %s", e.PaymentID, e.Status),
		fmt.Sprintf("Payment %s for merchant %s: amount %d, status %s.", e.PaymentID, e.MerchantID, e.Amount, e.Status))
}

// Consumer delivers events synchronously, one in-flight message at
// a time. Shutdown via ctx finishes the current delivery, then returns;
// unacked work is requeued by the broker.
type Consumer struct {
	ch          *amqp.Channel
	merchants   MerchantLookup
	notify      notifier
	queue       string
	route       string
	log         *slog.Logger
	maxAttempts int
	backoff     time.Duration
}

func NewConsumer(ch *amqp.Channel, merchants MerchantLookup, maxAttempts int, backoff time.Duration) *Consumer {
	return &Consumer{
		ch: ch, merchants: merchants,
		notify:      webhookNotifier{client: apiclient.New(nil)},
		queue:       DeliverQueue,
		route:       RouteKey,
		log:         slog.Default(),
		maxAttempts: maxAttempts,
		backoff:     backoff,
	}
}

func NewEmailConsumer(ch *amqp.Channel, merchants MerchantLookup, sender EmailSender, maxAttempts int, backoff time.Duration) *Consumer {
	if sender == nil {
		sender = NewLogEmailSender(nil)
	}
	return &Consumer{
		ch: ch, merchants: merchants,
		notify:      emailNotifier{sender: sender},
		queue:       EmailQueue,
		route:       EmailRoute,
		log:         slog.Default(),
		maxAttempts: maxAttempts,
		backoff:     backoff,
	}
}

// Run consumes until ctx is cancelled or the channel closes.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("%s consumer: qos: %w", c.notify.kind(), err)
	}

	deliveries, err := c.ch.Consume(c.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("%s consumer: consume: %w", c.notify.kind(), err)
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
	kind := c.notify.kind()
	var e Event
	if err := json.Unmarshal(d.Body, &e); err != nil {
		c.log.WarnContext(ctx, kind+" event unparseable, dead-lettering",
			slog.String("error", err.Error()))
		_ = d.Nack(false, false) // requeue=false routes to the DLQ via the DLX
		return
	}
	log := c.log.With(slog.String("payment_id", e.PaymentID), slog.String("merchant_id", e.MerchantID))

	m, err := c.merchants.FindByID(ctx, e.MerchantID)
	if err != nil {
		if err == merchant.ErrNotFound {
			log.WarnContext(ctx, kind+" merchant gone, dead-lettering")
			_ = d.Nack(false, false)
			return
		}
		log.WarnContext(ctx, kind+" merchant lookup failed, requeueing",
			slog.String("error", err.Error()))
		c.retryOrDeadLetter(ctx, log, d, e, err)
		return
	}
	if target := c.notify.target(m); target == "" {
		log.InfoContext(ctx, kind+" merchant has no target, skipping")
		_ = d.Ack(false)
		return
	} else if err := c.notify.send(ctx, target, d.Body); err != nil {
		log.WarnContext(ctx, kind+" delivery failed, requeueing",
			slog.String("error", err.Error()))
		c.retryOrDeadLetter(ctx, log, d, e, err)
		return
	}
	_ = d.Ack(false)
}

// retryOrDeadLetter republishes the message with an incremented attempt
// count after backoff, or dead-letters it past the cap. Shutdown mid-backoff
// requeues the original, so no event is lost.
func (c *Consumer) retryOrDeadLetter(ctx context.Context, log *slog.Logger, d amqp.Delivery, e Event, cause error) {
	kind := c.notify.kind()
	attempts := headerAttempts(d.Headers) + 1
	if attempts >= c.maxAttempts {
		log.WarnContext(ctx, kind+" attempts exhausted, dead-lettering",
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
		log.WarnContext(ctx, kind+" requeue failed, requeueing original",
			slog.String("error", err.Error()))
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}

func (c *Consumer) publish(ctx context.Context, body []byte, headers amqp.Table) error {
	return c.ch.PublishWithContext(ctx, Exchange, c.route, false, false, amqp.Publishing{
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
