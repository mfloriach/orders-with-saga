package payment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/webhooks"
)

type fakePayments struct {
	existing  *domain.Payment
	enqueued  []string
	authorize func(p *domain.Payment) error
	capture   func(p *domain.Payment) error
}

func (f *fakePayments) InsertPending(_ context.Context, p *domain.Payment) error {
	p.ID = "pay-test"
	p.Status = domain.StatusPending
	return nil
}

func (f *fakePayments) FindByIdem(_ context.Context, _, _ string) (*domain.Payment, error) {
	return f.existing, nil
}

func (f *fakePayments) MarkFailed(_ context.Context, p *domain.Payment) error {
	p.Status = domain.StatusFailed
	return nil
}

func (f *fakePayments) MarkAuthorized(_ context.Context, p *domain.Payment) error {
	if f.authorize != nil {
		return f.authorize(p)
	}
	p.Status = domain.StatusAuthorized
	return nil
}

func (f *fakePayments) MarkCaptured(_ context.Context, p *domain.Payment) error {
	if f.capture != nil {
		return f.capture(p)
	}
	p.Status = domain.StatusCaptured
	return nil
}

func (f *fakePayments) EnqueueReconciliation(_ context.Context, paymentID, _, _ string, _ error) error {
	f.enqueued = append(f.enqueued, paymentID)
	return nil
}

type fakeAudit struct {
	steps []domain.StepRecord
}

func (f *fakeAudit) RecordStep(_ context.Context, r domain.StepRecord) error {
	f.steps = append(f.steps, r)
	return nil
}

type fakeMerchants struct{ err error }

func (f *fakeMerchants) FindByID(_ context.Context, id string) (*domain.Merchant, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Merchant{ID: id}, nil
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

func testService(p *fakePayments, n *fakeNetwork, pub *webhooks.FakePublisher, epub *webhooks.FakeEmailPublisher, a *fakeAudit) *Service {
	return NewService(p, a, &fakeMerchants{}, nil, n, pub, epub)
}

// TestCreatePublishesWebhookEvent verifies one event with the payment's
// identity follows a successful creation.
func TestCreatePublishesWebhookEvent(t *testing.T) {
	payments := &fakePayments{}
	pub := &webhooks.FakePublisher{}
	svc := testService(payments, &fakeNetwork{}, pub, &webhooks.FakeEmailPublisher{}, &fakeAudit{})

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
// returns an error and emits no event.
func TestCreateFailurePublishesNothing(t *testing.T) {
	payments := &fakePayments{}
	pub := &webhooks.FakePublisher{}
	svc := testService(payments, &fakeNetwork{authErr: errors.New("refused")}, pub, &webhooks.FakeEmailPublisher{}, &fakeAudit{})

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
	svc := testService(payments, &fakeNetwork{captureErr: errors.New("timeout")}, pub, &webhooks.FakeEmailPublisher{}, &fakeAudit{})

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
	existing := &domain.Payment{ID: "pay-old", MerchantID: "m-1"}
	payments := &fakePayments{existing: existing}
	pub := &webhooks.FakePublisher{}
	svc := testService(payments, &fakeNetwork{}, pub, &webhooks.FakeEmailPublisher{}, &fakeAudit{})

	if _, err := svc.Create(context.Background(), "m-1", 500, "k-1"); err == nil {
		t.Fatal("expected conflict")
	}
	if len(pub.Events) != 0 {
		t.Fatalf("expected no events, got %+v", pub.Events)
	}
}

// TestCreateRecordsAuditTrail verifies one success row per executed step.
func TestCreateRecordsAuditTrail(t *testing.T) {
	payments := &fakePayments{}
	audit := &fakeAudit{}
	svc := testService(payments, &fakeNetwork{}, &webhooks.FakePublisher{}, &webhooks.FakeEmailPublisher{}, audit)

	p, err := svc.Create(context.Background(), "m-1", 500, "k-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	want := []string{"insert-payment", "authorize", "call-api-capture", "send-webhook"}
	if len(audit.steps) != len(want) {
		t.Fatalf("expected %d audit rows, got %+v", len(want), audit.steps)
	}
	for i, name := range want {
		got := audit.steps[i]
		if got.Step != name || got.Outcome != "success" || got.Reason != "" {
			t.Fatalf("bad row %d: %+v", i, got)
		}
		if got.PaymentID != p.ID || got.SagaID == "" {
			t.Fatalf("bad identity in row %d: %+v", i, got)
		}
	}
}

// TestCreateFailureRecordsReason verifies the failing step's row carries
// the error text while earlier steps record success.
func TestCreateFailureRecordsReason(t *testing.T) {
	payments := &fakePayments{}
	audit := &fakeAudit{}
	svc := testService(payments, &fakeNetwork{authErr: errors.New("refused")}, &webhooks.FakePublisher{}, &webhooks.FakeEmailPublisher{}, audit)

	if _, err := svc.Create(context.Background(), "m-1", 500, "k-1"); err == nil {
		t.Fatal("expected error")
	}
	if len(audit.steps) != 2 {
		t.Fatalf("expected 2 audit rows, got %+v", audit.steps)
	}
	if audit.steps[0].Step != "insert-payment" || audit.steps[0].Outcome != "success" {
		t.Fatalf("bad first row: %+v", audit.steps[0])
	}
	last := audit.steps[1]
	if last.Step != "authorize" || last.Outcome != "failure" {
		t.Fatalf("bad failure row: %+v", last)
	}
	if !strings.Contains(last.Reason, "refused") {
		t.Fatalf("reason missing cause: %+v", last)
	}
}

// TestCreateEmailPublishesOnSuccess verifies one email event with the
// payment's identity follows a successful creation.
func TestCreateEmailPublishesOnSuccess(t *testing.T) {
	payments := &fakePayments{}
	emails := &webhooks.FakeEmailPublisher{}
	svc := testService(payments, &fakeNetwork{}, &webhooks.FakePublisher{}, emails, &fakeAudit{})

	p, err := svc.Create(context.Background(), "m-1", 500, "k-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(emails.Events) != 1 {
		t.Fatalf("expected 1 email event, got %d", len(emails.Events))
	}
	got := emails.Events[0]
	if got.PaymentID != p.ID || got.MerchantID != "m-1" || got.Amount != 500 {
		t.Fatalf("bad email event: %+v for payment %+v", got, p)
	}
}

// TestCreateEmailPublishesNothingOnFailure verifies a failed creation emits
// no email event.
func TestCreateEmailPublishesNothingOnFailure(t *testing.T) {
	payments := &fakePayments{}
	emails := &webhooks.FakeEmailPublisher{}
	svc := testService(payments, &fakeNetwork{authErr: errors.New("refused")}, &webhooks.FakePublisher{}, emails, &fakeAudit{})

	if _, err := svc.Create(context.Background(), "m-1", 500, "k-1"); err == nil {
		t.Fatal("expected error")
	}
	if len(emails.Events) != 0 {
		t.Fatalf("expected no email events, got %+v", emails.Events)
	}
}

// TestEmailPublishFailureFailsLoudly pins the accepted posture: the saga
// steps already succeeded, so there is nothing to unwind — the error
// surfaces to the caller.
func TestEmailPublishFailureFailsLoudly(t *testing.T) {
	payments := &fakePayments{}
	emails := &webhooks.FakeEmailPublisher{Err: errors.New("broker down")}
	svc := testService(payments, &fakeNetwork{}, &webhooks.FakePublisher{}, emails, &fakeAudit{})

	if _, err := svc.Create(context.Background(), "m-1", 500, "k-1"); err == nil {
		t.Fatal("expected publish error")
	}
}
