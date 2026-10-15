package payment

import (
	"context"
	"errors"
	"testing"

	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment/models"
	"paymentgateway/internal/webhooks"
)

type fakePayments struct {
	existing   *models.Payment
	enqueued   []string
	authorize  func(p *models.Payment) error
	capture    func(p *models.Payment) error
}

func (f *fakePayments) InsertPending(_ context.Context, p *models.Payment) error {
	p.ID = "pay-test"
	p.Status = models.StatusPending
	return nil
}

func (f *fakePayments) FindByIdem(_ context.Context, _, _ string) (*models.Payment, error) {
	return f.existing, nil
}

func (f *fakePayments) MarkFailed(_ context.Context, p *models.Payment) error {
	p.Status = models.StatusFailed
	return nil
}

func (f *fakePayments) MarkAuthorized(_ context.Context, p *models.Payment) error {
	if f.authorize != nil {
		return f.authorize(p)
	}
	p.Status = models.StatusAuthorized
	return nil
}

func (f *fakePayments) MarkCaptured(_ context.Context, p *models.Payment) error {
	if f.capture != nil {
		return f.capture(p)
	}
	p.Status = models.StatusCaptured
	return nil
}

func (f *fakePayments) EnqueueReconciliation(_ context.Context, paymentID, _, _ string, _ error) error {
	f.enqueued = append(f.enqueued, paymentID)
	return nil
}

type fakeAudit struct{}

func (fakeAudit) Insert(_ context.Context, _ string, _ int64) error { return nil }
func (fakeAudit) MarkFailed(_ context.Context, _ string) error      { return nil }

type fakeMerchants struct{ err error }

func (f *fakeMerchants) FindByID(_ context.Context, id string) (*merchant.Merchant, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &merchant.Merchant{ID: id}, nil
}

type fakeNetwork struct {
	authErr    error
	captureErr error
}

func (f *fakeNetwork) Authenticate(_ context.Context, _ string, _ int64, _ string) error {
	return f.authErr
}

func (f *fakeNetwork) Capture(_ context.Context, _ string, _ int64, _ string) error {
	return f.captureErr
}

func (f *fakeNetwork) GetPayment(_ context.Context, _ string) (string, string, error) {
	return "", "", nil
}

func testService(p *fakePayments, n *fakeNetwork, pub *webhooks.FakePublisher) *Service {
	return NewService(p, fakeAudit{}, &fakeMerchants{}, nil, n, pub)
}

// TestCreatePublishesWebhookEvent verifies one event with the payment's
// identity follows a successful creation.
func TestCreatePublishesWebhookEvent(t *testing.T) {
	payments := &fakePayments{}
	pub := &webhooks.FakePublisher{}
	svc := testService(payments, &fakeNetwork{}, pub)

	p, err := svc.Create(context.Background(), "m-1", 500, "k-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(pub.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.Events))
	}
	got := pub.Events[0]
	if got.PaymentID != p.ID || got.MerchantID != "m-1" || got.Amount != 500 || got.Status != string(p.Status) {
		t.Fatalf("bad event: %+v for payment %+v", got, p)
	}
}

// TestCreateFailurePublishesNothing verifies a refused network call
// compensates and emits no event.
func TestCreateFailurePublishesNothing(t *testing.T) {
	payments := &fakePayments{}
	pub := &webhooks.FakePublisher{}
	svc := testService(payments, &fakeNetwork{authErr: errors.New("refused")}, pub)

	if _, err := svc.Create(context.Background(), "m-1", 500, "k-1"); err == nil {
		t.Fatal("expected error")
	}
	if len(pub.Events) != 0 {
		t.Fatalf("expected no events, got %+v", pub.Events)
	}
}

// TestCreateInDoubtPublishesNothing verifies an unconfirmed capture keeps
// the payment pending via the outbox path and emits no event.
func TestCreateInDoubtPublishesNothing(t *testing.T) {
	payments := &fakePayments{}
	pub := &webhooks.FakePublisher{}
	svc := testService(payments, &fakeNetwork{captureErr: errors.New("timeout")}, pub)

	if _, err := svc.Create(context.Background(), "m-1", 500, "k-1"); err == nil {
		t.Fatal("expected in-doubt error")
	}
	if len(pub.Events) != 0 {
		t.Fatalf("expected no events, got %+v", pub.Events)
	}
	if len(payments.enqueued) != 1 {
		t.Fatalf("expected outbox enqueue, got %v", payments.enqueued)
	}
}

// TestCreateConflictPublishesNothing verifies a duplicate idempotency key
// returns the conflict without emitting an event.
func TestCreateConflictPublishesNothing(t *testing.T) {
	existing := &models.Payment{ID: "pay-old", MerchantID: "m-1"}
	payments := &fakePayments{existing: existing}
	pub := &webhooks.FakePublisher{}
	svc := testService(payments, &fakeNetwork{}, pub)

	if _, err := svc.Create(context.Background(), "m-1", 500, "k-1"); err == nil {
		t.Fatal("expected conflict")
	}
	if len(pub.Events) != 0 {
		t.Fatalf("expected no events, got %+v", pub.Events)
	}
}
