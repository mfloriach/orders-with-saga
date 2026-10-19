package workflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"paymentgateway/internal/domain"
	"paymentgateway/pkg/apiclient"
)

type stubMerchantLookup struct {
	merchant *domain.Merchant
	err      error
}

func (s stubMerchantLookup) FindByID(_ context.Context, _ string) (*domain.Merchant, error) {
	return s.merchant, s.err
}

func TestWebhookStepDeliversOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := apiclient.New(nil,
		apiclient.WithRetry(3, time.Millisecond, 5*time.Millisecond, nil),
		apiclient.WithTimeout(2*time.Second),
	)
	lookup := stubMerchantLookup{
		merchant: &domain.Merchant{ID: "m-1", WebhookURL: strPtr(srv.URL)},
	}
	w := webhook{Merchants: &lookup, Client: client}

	pw := &PaymentWorkflow{Pending: &domain.PaymentPending{
		ID:             "p-1",
		MerchantID:     "m-1",
		Amount:         1000,
		IdempotencyKey: "idem-1",
		Status:         domain.StatusAuthorized,
	}}

	if err := w.Do(context.Background(), pw); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestWebhookStepFailsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	client := apiclient.New(nil,
		apiclient.WithRetry(1, 0, 0, nil),
		apiclient.WithTimeout(2*time.Second),
	)
	lookup := stubMerchantLookup{
		merchant: &domain.Merchant{ID: "m-1", WebhookURL: strPtr(srv.URL)},
	}
	w := webhook{Merchants: &lookup, Client: client}

	pw := &PaymentWorkflow{Pending: &domain.PaymentPending{
		ID:             "p-1",
		MerchantID:     "m-1",
		Amount:         1000,
		IdempotencyKey: "idem-1",
		Status:         domain.StatusAuthorized,
	}}

	if err := w.Do(context.Background(), pw); err == nil {
		t.Fatal("expected error from non-2xx merchant response, got nil")
	}
}

func TestWebhookStepSkipsWithoutURL(t *testing.T) {
	client := apiclient.New(nil)
	lookup := stubMerchantLookup{
		merchant: &domain.Merchant{ID: "m-1"},
	}
	w := webhook{Merchants: &lookup, Client: client}

	pw := &PaymentWorkflow{Pending: &domain.PaymentPending{
		ID:             "p-1",
		MerchantID:     "m-1",
		Amount:         1000,
		IdempotencyKey: "idem-1",
		Status:         domain.StatusAuthorized,
	}}

	if err := w.Do(context.Background(), pw); err != nil {
		t.Fatalf("expected skip with no webhook URL, got %v", err)
	}
}

func strPtr(s string) *string { return &s }
