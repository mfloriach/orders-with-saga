package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"paymentgateway/internal/merchant"
	"paymentgateway/pkg/apperr"
)

type stubService struct {
	create func(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error)
}

func (s stubService) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error) {
	return s.create(ctx, merchantID, amount, idemKey)
}

const testMerchantID = "123e4567-e89b-12d3-a456-426614174000"

func TestCreateErrors(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		stub       func(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error)
		wantStatus int
		wantCode   string
		wantExtra  string
	}{
		{"invalid json", `{bad`, nil, 400, "validation_error", ""},
		{"validation failure", `{"merchant_id":"not-a-uuid","amount":0}`, nil, 400, "validation_error", ""},
		{
			"merchant not found",
			`{"merchant_id":"` + testMerchantID + `","amount":100,"idempotency_key":"k"}`,
			func(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error) {
				return nil, merchant.ErrNotFound
			},
			404, "merchant_not_found", "",
		},
		{
			"idempotency conflict",
			`{"merchant_id":"` + testMerchantID + `","amount":100,"idempotency_key":"k"}`,
			func(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error) {
				return nil, &ConflictError{Existing: &Payment{ID: "pid-9"}}
			},
			409, "idempotency_conflict", "pid-9",
		},
		{
			"internal error",
			`{"merchant_id":"` + testMerchantID + `","amount":100,"idempotency_key":"k"}`,
			func(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error) {
				return nil, errors.New("boom")
			},
			500, "internal_error", "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(stubService{create: tc.stub})
			app := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})
			app.Post("/payments", h.Create)
			req := httptest.NewRequest("POST", "/payments", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["code"] != tc.wantCode {
				t.Fatalf("code: got %v, want %v", body["code"], tc.wantCode)
			}
			if tc.wantExtra != "" && body["payment_id"] != tc.wantExtra {
				t.Fatalf("payment_id: got %v, want %v", body["payment_id"], tc.wantExtra)
			}
		})
	}
}

func TestCreateSuccess(t *testing.T) {
	h := NewHandler(stubService{create: func(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error) {
		return &Payment{ID: "p-1", MerchantID: merchantID, Amount: amount, IdempotencyKey: idemKey, Status: "pending"}, nil
	}})
	app := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})
	app.Post("/payments", h.Create)
	req := httptest.NewRequest("POST", "/payments", strings.NewReader(`{"merchant_id":"`+testMerchantID+`","amount":100,"idempotency_key":"k"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status: got %d, want 201", resp.StatusCode)
	}
}
