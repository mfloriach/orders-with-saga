package webhooks

import (
	"context"
	"strings"
	"testing"
	"time"

	"paymentgateway/internal/domain"

	amqp "github.com/rabbitmq/amqp091-go"
)

func runEmailConsumer(t *testing.T, ch *amqp.Channel, lookup MerchantLookup, sender *FakeSender) context.CancelFunc {
	t.Helper()
	c := NewEmailConsumer(ch, lookup, sender, 2, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = c.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return cancel
}

// TestEmailConsumerDelivers verifies the sender receives the merchant email
// address with payment details and the event is acknowledged.
func TestEmailConsumerDelivers(t *testing.T) {
	consumeCh, ch := liveChannel(t)
	sender := &FakeSender{}
	lookup := &fakeLookup{m: &domain.Merchant{ID: "m-1", Email: "m1@example.com"}}
	runEmailConsumer(t, consumeCh, lookup, sender)
	publishEmailEvent(t, ch, Event{PaymentID: "p-mail", MerchantID: "m-1", Amount: 100, Status: "captured"})

	waitFor(t, "email delivery", func() bool { return len(sender.sent()) == 1 })
	got := sender.sent()[0]
	if got.To != "m1@example.com" {
		t.Fatalf("bad recipient: %+v", got)
	}
	for _, want := range []string{"p-mail", "100", "captured"} {
		if !strings.Contains(got.Subject+got.Body, want) {
			t.Fatalf("payload missing %q: %+v", want, got)
		}
	}
	waitFor(t, "ack", func() bool { return queueDepth(t, ch, EmailQueue) == 0 })
}

// TestEmailConsumerSkipsWithoutEmail verifies a merchant with no email
// address acks the event with no send attempt.
func TestEmailConsumerSkipsWithoutEmail(t *testing.T) {
	consumeCh, ch := liveChannel(t)
	sender := &FakeSender{}
	lookup := &fakeLookup{m: &domain.Merchant{ID: "m-2"}}
	runEmailConsumer(t, consumeCh, lookup, sender)
	publishEmailEvent(t, ch, Event{PaymentID: "p-skip-mail", MerchantID: "m-2", Amount: 50, Status: "captured"})

	waitFor(t, "ack", func() bool { return queueDepth(t, ch, EmailQueue) == 0 })
	if n := sender.calls(); n != 0 {
		t.Fatalf("sender called %d times for merchant without email", n)
	}
}

// TestEmailConsumerRetriesThenDeadLetters verifies persistent sender
// failures are retried and then parked in the email DLQ.
func TestEmailConsumerRetriesThenDeadLetters(t *testing.T) {
	consumeCh, ch := liveChannel(t)
	sender := &FakeSender{Err: context.DeadlineExceeded}
	lookup := &fakeLookup{m: &domain.Merchant{ID: "m-3", Email: "m3@example.com"}}
	runEmailConsumer(t, consumeCh, lookup, sender)
	publishEmailEvent(t, ch, Event{PaymentID: "p-mail-dlq", MerchantID: "m-3", Amount: 75, Status: "captured"})

	waitFor(t, "dlq parking", func() bool { return queueDepth(t, ch, EmailDLQueue) == 1 })
	if n := sender.calls(); n < 2 {
		t.Fatalf("expected a retry before DLQ, got %d calls", n)
	}
	if n := queueDepth(t, ch, EmailQueue); n != 0 {
		t.Fatalf("email queue not drained: %d", n)
	}
	drainQueue(t, ch, EmailDLQueue)
}

// TestEmailConsumerMalformedDeadLetters verifies an unparseable message is
// parked without send attempts.
func TestEmailConsumerMalformedDeadLetters(t *testing.T) {
	consumeCh, ch := liveChannel(t)
	sender := &FakeSender{}
	lookup := &fakeLookup{m: &domain.Merchant{ID: "m-4", Email: "m4@example.com"}}
	runEmailConsumer(t, consumeCh, lookup, sender)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ch.PublishWithContext(ctx, Exchange, EmailRoute, false, false, amqp.Publishing{
		ContentType: "text/plain", Body: []byte("not-json{{{"),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	waitFor(t, "dlq parking", func() bool { return queueDepth(t, ch, EmailDLQueue) == 1 })
	if n := sender.calls(); n != 0 {
		t.Fatalf("sender called %d times for malformed event", n)
	}
	drainQueue(t, ch, EmailDLQueue)
}
