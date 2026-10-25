package relay

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"paymentgateway/internal/domain"
)

type fakeBroker struct {
	published []CapturedEvent
	err       error
}

func (f *fakeBroker) Publish(_ context.Context, routingKey string, body []byte) error {
	if f.err != nil {
		return f.err
	}
	var e CapturedEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return err
	}
	f.published = append(f.published, e)
	return nil
}

type fakeOutbox struct {
	rows           []*domain.OutboxPayment
	markProcessed  []string
	markFailed     []string
	incrementCalls []string
}

func (f *fakeOutbox) Store(_ context.Context, _ domain.PaymentAuthenticated) error { return nil }
func (f *fakeOutbox) GetByStatus(_ context.Context) ([]*domain.OutboxPayment, error) {
	return f.rows, nil
}
func (f *fakeOutbox) MarkProcessed(_ context.Context, outboxID string) error {
	f.markProcessed = append(f.markProcessed, outboxID)
	return nil
}
func (f *fakeOutbox) MarkFailed(_ context.Context, outboxID string, _ string) error {
	f.markFailed = append(f.markFailed, outboxID)
	return nil
}
func (f *fakeOutbox) IncrementAttempts(_ context.Context, outboxID string, _ string) error {
	f.incrementCalls = append(f.incrementCalls, outboxID)
	return nil
}

func newWorker(o *fakeOutbox, b Broker, maxAttempts int) *Worker {
	return NewWorker(o, b, maxAttempts, 0)
}

func TestRelayProcessPublishesAndMarksProcessed(t *testing.T) {
	o := &fakeOutbox{rows: []*domain.OutboxPayment{{
		ID: "outbox-1", PaymentID: "pay-1", MerchantID: "mer-1",
		Status: domain.OutboxStatusPending, Attempts: 0,
	}}}
	b := &fakeBroker{}
	w := newWorker(o, b, 3)

	w.process(context.Background(), o.rows[0])

	if len(b.published) != 1 || b.published[0].PaymentID != "pay-1" {
		t.Fatalf("expected one published event with paymentID pay-1, got %+v", b.published)
	}
	if len(o.markProcessed) != 1 || o.markProcessed[0] != "outbox-1" {
		t.Fatalf("expected markProcessed outbox-1, got %v", o.markProcessed)
	}
}

func TestRelayProcessRetriesThenMarksFailed(t *testing.T) {
	o := &fakeOutbox{rows: []*domain.OutboxPayment{{
		ID: "outbox-2", PaymentID: "pay-2", MerchantID: "mer-2",
		Status: domain.OutboxStatusPending, Attempts: 2,
	}}}
	b := &fakeBroker{err: errors.New("broker down")}
	w := newWorker(o, b, 3)

	w.process(context.Background(), o.rows[0])

	if len(o.incrementCalls) != 1 || o.incrementCalls[0] != "outbox-2" {
		t.Fatalf("expected incrementAttempts outbox-2, got %v", o.incrementCalls)
	}
	if len(o.markFailed) != 1 || o.markFailed[0] != "outbox-2" {
		t.Fatalf("expected markFailed outbox-2 at attempt bound, got %v", o.markFailed)
	}
	if len(o.markProcessed) != 0 {
		t.Fatalf("expected no markProcessed, got %v", o.markProcessed)
	}
}

func TestRelayProcessRetriesBelowBoundStaysPending(t *testing.T) {
	o := &fakeOutbox{rows: []*domain.OutboxPayment{{
		ID: "outbox-3", PaymentID: "pay-3", MerchantID: "mer-3",
		Status: domain.OutboxStatusPending, Attempts: 0,
	}}}
	b := &fakeBroker{err: errors.New("broker down")}
	w := newWorker(o, b, 3)

	w.process(context.Background(), o.rows[0])

	if len(o.markFailed) != 0 || len(o.markProcessed) != 0 {
		t.Fatalf("expected no terminal marking below bound, got processed=%v failed=%v", o.markProcessed, o.markFailed)
	}
}
