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

	"paymentgateway/internal/domain"

	amqp "github.com/rabbitmq/amqp091-go"
)

type fakeLookup struct {
	mu  sync.Mutex
	m   *domain.Merchant
	err error
}

func (f *fakeLookup) FindByID(_ context.Context, id string) (*domain.Merchant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.m, nil
}

func strptr(s string) *string { return &s }

// liveChannel opens a broker connection for consumer tests, declares the
// topology, purges all queues for isolation, and returns two channels: the
// first is owned by the consumer under test, the second is for test
// assertions. They must be separate because concurrent RPC on one amqp091
// channel (consumer ack/nack plus polling inspects) interleaves frames and
// gets the channel closed by the server. Skips without a live broker.
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
	declareTestTopology(t, assertCh)
	for _, q := range []string{DeliverQueue, DLQueue, EmailQueue, EmailDLQueue} {
		if _, err := assertCh.QueuePurge(q, false); err != nil {
			t.Fatalf("purge %s: %v", q, err)
		}
	}
	return consumeCh, assertCh
}

// declareTestTopology mirrors the broker's boot declaration so tests never
// depend on declaration order. QueueDeclare is idempotent for identical
// arguments.
func declareTestTopology(t *testing.T, ch *amqp.Channel) {
	t.Helper()
	if err := ch.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil); err != nil {
		t.Fatalf("declare exchange: %v", err)
	}
	if err := ch.ExchangeDeclare(DLX, "topic", true, false, false, false, nil); err != nil {
		t.Fatalf("declare dlx: %v", err)
	}
	declare := func(queue, bindKey, exchange string, dlx bool) {
		args := amqp.Table{}
		if dlx {
			args["x-dead-letter-exchange"] = DLX
		}
		if _, err := ch.QueueDeclare(queue, true, false, false, false, args); err != nil {
			t.Fatalf("declare %s: %v", queue, err)
		}
		if err := ch.QueueBind(queue, bindKey, exchange, false, nil); err != nil {
			t.Fatalf("bind %s: %v", queue, err)
		}
	}
	declare(DeliverQueue, "webhook.*", Exchange, true)
	declare(DLQueue, "webhook.*", DLX, false)
	declare(EmailQueue, "email.*", Exchange, true)
	declare(EmailDLQueue, "email.*", DLX, false)
}

func publishEvent(t *testing.T, ch *amqp.Channel, e Event) {
	publishRoutedEvent(t, ch, RouteKey, e)
}

func publishEmailEvent(t *testing.T, ch *amqp.Channel, e Event) {
	publishRoutedEvent(t, ch, EmailRoute, e)
}

func publishRoutedEvent(t *testing.T, ch *amqp.Channel, route string, e Event) {
	t.Helper()
	body, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ch.PublishWithContext(ctx, Exchange, route, false, false, amqp.Publishing{
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

func drainQueue(t *testing.T, ch *amqp.Channel, queue string) {
	t.Helper()
	for {
		msg, ok, err := ch.Get(queue, false)
		if err != nil || !ok {
			return
		}
		_ = ch.Ack(msg.DeliveryTag, false)
	}
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

	lookup := &fakeLookup{m: &domain.Merchant{ID: "m-1", WebhookURL: strptr(srv.URL)}}
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

	lookup := &fakeLookup{m: &domain.Merchant{ID: "m-2"}} // no WebhookURL
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

	lookup := &fakeLookup{m: &domain.Merchant{ID: "m-3", WebhookURL: strptr(srv.URL)}}
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
	drainQueue(t, ch, DLQueue)
}

// TestConsumerMalformedDeadLetters verifies an unparseable message is
// parked without retries.
func TestConsumerMalformedDeadLetters(t *testing.T) {
	consumeCh, ch := liveChannel(t)
	lookup := &fakeLookup{m: &domain.Merchant{ID: "m-4", WebhookURL: strptr("http://example.invalid")}}
	runConsumer(t, consumeCh, lookup)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ch.PublishWithContext(ctx, Exchange, RouteKey, false, false, amqp.Publishing{
		ContentType: "text/plain", Body: []byte("not-json{{{"),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	waitFor(t, "dlq parking", func() bool { return queueDepth(t, ch, DLQueue) == 1 })
	drainQueue(t, ch, DLQueue)
}
