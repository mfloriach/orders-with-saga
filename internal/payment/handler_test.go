package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment/models"
	"paymentgateway/pkg/apperr"

	"github.com/gofiber/fiber/v2"
)

type stubService struct {
	create func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error)
}

func (s stubService) Create(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error) {
	return s.create(ctx, merchantID, amount, idemHeader)
}

const testMerchantID = "123e4567-e89b-12d3-a456-426614174000"

func doPost(t *testing.T, h *Handler, body, idemHeader string) (int, map[string]any, string) {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})
	app.Post("/payments", h.Create)
	req := httptest.NewRequest("POST", "/payments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if idemHeader != "" {
		req.Header.Set("Idempotency-Key", idemHeader)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, decoded, resp.Header.Get("Idempotency-Key")
}

func okStub() func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error) {
	return func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error) {
		return &models.Payment{ID: "p-1", MerchantID: merchantID, Amount: amount, IdempotencyKey: idemHeader, Status: "pending"}, nil
	}
}

func TestCreateErrors(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		stub       func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error)
		wantStatus int
		wantCode   string
		wantExtra  string
	}{
		{"invalid json", `{bad`, nil, 400, "validation_error", ""},
		{"validation failure", `{"merchant_id":"not-a-uuid","amount":0}`, nil, 400, "validation_error", ""},
		{
			"merchant not found",
			`{"merchant_id":"` + testMerchantID + `","amount":100}`,
			func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error) {
				return nil, merchant.ErrNotFound
			},
			404, "merchant_not_found", "",
		},
		{
			"idempotency conflict",
			`{"merchant_id":"` + testMerchantID + `","amount":100}`,
			func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error) {
				return nil, &ConflictError{Existing: &models.Payment{ID: "pid-9", IdempotencyKey: "k-1"}}
			},
			409, "idempotency_conflict", "pid-9",
		},
		{
			"internal error",
			`{"merchant_id":"` + testMerchantID + `","amount":100}`,
			func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error) {
				return nil, errors.New("boom")
			},
			500, "internal_error", "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body, _ := doPost(t, NewHandler(stubService{create: tc.stub}), tc.body, "k-1")
			if status != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", status, tc.wantStatus)
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
	status, _, echo := doPost(t,
		NewHandler(stubService{create: okStub()}),
		`{"merchant_id":"`+testMerchantID+`","amount":100}`, "k-1")
	if status != 201 {
		t.Fatalf("status: got %d, want 201", status)
	}
	if echo != "k-1" {
		t.Fatalf("echo header: got %q, want %q", echo, "k-1")
	}
}

// TestCreateEchoesStoredStatus pins that 201 echoes the service status,
// authorized once the network confirms funds held.
func TestCreateEchoesStoredStatus(t *testing.T) {
	status, body, _ := doPost(t,
		NewHandler(stubService{create: func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error) {
			return &models.Payment{ID: "p-1", MerchantID: merchantID, Amount: amount, IdempotencyKey: idemHeader, Status: models.StatusAuthorized}, nil
		}}),
		`{"merchant_id":"`+testMerchantID+`","amount":100}`, "k-1")
	if status != 201 || body["status"] != string(models.StatusAuthorized) {
		t.Fatalf("got %d %v", status, body)
	}
}

func TestHeaderPassedToService(t *testing.T) {
	var gotHeader string
	h := NewHandler(stubService{create: func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error) {
		gotHeader = idemHeader
		return &models.Payment{ID: "p-1", IdempotencyKey: idemHeader}, nil
	}})
	status, _, echo := doPost(t, h, `{"merchant_id":"`+testMerchantID+`","amount":100}`, "client-key")
	if status != 201 || gotHeader != "client-key" {
		t.Fatalf("got header=%q status=%d", gotHeader, status)
	}
	if echo != "client-key" {
		t.Fatalf("echo header: got %q", echo)
	}
}

func TestMissingHeaderEchoesEffectiveKey(t *testing.T) {
	// ponytail follow-up: derivation hashes the raw RequestURI, so every
	// header-less POST /payments shares one key per merchant — collides on
	// repeat purchases. Tracked separately; this test pins current behavior.
	h := NewHandler(stubService{create: func(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error) {
		if !strings.HasPrefix(idemHeader, "drv:") {
			t.Fatalf("expected derived key, got %q", idemHeader)
		}
		return &models.Payment{ID: "p-1", IdempotencyKey: idemHeader}, nil
	}})
	status, _, echo := doPost(t, h, `{"merchant_id":"`+testMerchantID+`","amount":100}`, "")
	if status != 201 {
		t.Fatalf("status: got %d, want 201", status)
	}
	if !strings.HasPrefix(echo, "drv:") {
		t.Fatalf("echo header: got %q, want derived key", echo)
	}
}
