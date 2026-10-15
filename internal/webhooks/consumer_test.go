package webhooks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"paymentgateway/internal/merchant"

	amqp "github.com/rabbitmq/amqp091-go"
)

type fakeLookup struct {
	mu  sync.Mutex
	m   *merchant.Merchant
	err error
}

func (f *fakeLookup) FindByID(_ context.Context, id string) (*merchant.Merchant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.m, nil
}

func strptr(s string) *string { return &s }

// liveChannel opens a broker connection for consumer tests, purging both
// queues for isolation, and returns two channels: the first is owned by
// the consumer under test, the second is for test assertions. They must be
// separate because concurrent RPC on one amqp091 channel (consumer
// ack/nack plus polling inspects) interleaves frames and gets the channel
// closed by the server. Skips without a live broker.
func liveChannel(t *testing.T) (consumeCh, assertCh *amqp.Channel) {
	t.Helper()
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://app:app@localhost:5672/"
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Skipf("no live rabbitmq: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	open := func() *amqp.Channel {
		ch, err := conn.Channel()
		if err != nil {
			t.Fatalf("open channel: %v", err)
		}
		t.Cleanup(func() { ch.Close() })
		return ch
	}
	consumeCh, assertCh = open(), open()
	for _, q := range []string{DeliverQueue, DLQueue} {
		if _, err := assertCh.QueuePurge(q, false); err != nil {
			t.Fatalf("purge %s: %v", q, err)
		}
	}
	return consumeCh, assertCh
}

func publishEvent(t *testing.T, ch *amqp.Channel, e Event) {
	t.Helper()
	body, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ch.PublishWithContext(ctx, Exchange, RouteKey, false, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body,
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

// queueDepth returns the ready message count for a queue.
func queueDepth(t *testing.T, ch *amqp.Channel, queue string) int {
	t.Helper()
	q, err := ch.QueueInspect(queue)
	if err != nil {
		t.Fatalf("inspect %s: %v", queue, err)
	}
	return q.Messages
}

// waitFor polls cond until true or the timeout, failing the test.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func runConsumer(t *testing.T, ch *amqp.Channel, lookup MerchantLookup) context.CancelFunc {
	t.Helper()
	c := NewConsumer(ch, lookup, 2, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = c.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return cancel
}

// TestConsumerDelivers verifies a 2xx merchant response acks the event.
func TestConsumerDelivers(t *testing.T) {
	consumeCh, ch := liveChannel(t)
	var mu sync.Mutex
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	lookup := &fakeLookup{m: &merchant.Merchant{ID: "m-1", WebhookURL: strptr(srv.URL)}}
	runConsumer(t, consumeCh, lookup)
	sent := Event{PaymentID: "p-deliver", MerchantID: "m-1", Amount: 100, Status: "captured"}
	publishEvent(t, ch, sent)

	waitFor(t, "delivery", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) > 0
	})
	var e Event
	mu.Lock()
	defer mu.Unlock()
	if err := json.Unmarshal(got, &e); err != nil || e != sent {
		t.Fatalf("bad payload %s: %v", got, err)
	}
	waitFor(t, "ack", func() bool { return queueDepth(t, ch, DeliverQueue) == 0 })
}

// TestConsumerSkipsWithoutURL verifies a merchant with no webhook URL acks
// the event with no HTTP call.
func TestConsumerSkipsWithoutURL(t *testing.T) {
	consumeCh, ch := liveChannel(t)
	called := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	lookup := &fakeLookup{m: &merchant.Merchant{ID: "m-2"}} // no WebhookURL
	runConsumer(t, consumeCh, lookup)
	publishEvent(t, ch, Event{PaymentID: "p-skip", MerchantID: "m-2", Amount: 50, Status: "captured"})

	waitFor(t, "ack", func() bool { return queueDepth(t, ch, DeliverQueue) == 0 })
	select {
	case <-called:
		t.Fatal("merchant endpoint was called for merchant without url")
	default:
	}
}

// TestConsumerRetriesThenDeadLetters verifies persistent 500s are retried
// and then parked in the DLQ.
func TestConsumerRetriesThenDeadLetters(t *testing.T) {
	consumeCh, ch := liveChannel(t)
	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	lookup := &fakeLookup{m: &merchant.Merchant{ID: "m-3", WebhookURL: strptr(srv.URL)}}
	runConsumer(t, consumeCh, lookup)
	publishEvent(t, ch, Event{PaymentID: "p-dlq", MerchantID: "m-3", Amount: 75, Status: "captured"})

	waitFor(t, "dlq parking", func() bool { return queueDepth(t, ch, DLQueue) == 1 })
	mu.Lock()
	defer mu.Unlock()
	if hits < 2 {
		t.Fatalf("expected a retry before DLQ, got %d hits", hits)
	}
	if n := queueDepth(t, ch, DeliverQueue); n != 0 {
		t.Fatalf("deliver queue not drained: %d", n)
	}
	// Drain the parked message so later tests start clean.
	msg, ok, err := ch.Get(DLQueue, false)
	if err != nil || !ok {
		t.Fatalf("get dlq: ok=%v err=%v", ok, err)
	}
	var e Event
	if err := json.Unmarshal(msg.Body, &e); err != nil || e.PaymentID != "p-dlq" {
		t.Fatalf("bad dlq payload %s: %v", msg.Body, err)
	}
	_ = ch.Ack(msg.DeliveryTag, false)
}

// TestConsumerMalformedDeadLetters verifies an unparseable message is
// parked without retries.
func TestConsumerMalformedDeadLetters(t *testing.T) {
	consumeCh, ch := liveChannel(t)
	lookup := &fakeLookup{m: &merchant.Merchant{ID: "m-4", WebhookURL: strptr("http://example.invalid")}}
	runConsumer(t, consumeCh, lookup)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ch.PublishWithContext(ctx, Exchange, RouteKey, false, false, amqp.Publishing{
		ContentType: "text/plain", Body: []byte("not-json{{{"),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	waitFor(t, "dlq parking", func() bool { return queueDepth(t, ch, DLQueue) == 1 })
	msg, ok, err := ch.Get(DLQueue, false)
	if err != nil || !ok {
		t.Fatalf("get dlq: ok=%v err=%v", ok, err)
	}
	_ = ch.Ack(msg.DeliveryTag, false)
}
