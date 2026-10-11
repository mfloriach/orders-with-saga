package apperr

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestHandler(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantExtra  map[string]any
	}{
		{"invalid json", InvalidJSON(), 400, "validation_error", map[string]any{"errors": map[string]any{"body": "invalid JSON"}}},
		{"validation", ValidationFailed(map[string]string{"email": "failed email validation"}), 400, "validation_error", map[string]any{"errors": map[string]any{"email": "failed email validation"}}},
		{"email taken", EmailTaken(), 409, "email_taken", nil},
		{"merchant not found", MerchantNotFound(), 404, "merchant_not_found", nil},
		{"idempotency conflict", IdempotencyConflict("pid-1"), 409, "idempotency_conflict", map[string]any{"payment_id": "pid-1"}},
		{"rate limited", RateLimitedError(60), 429, "rate_limited", map[string]any{"retry_after": float64(60)}},
		{"internal", InternalError(), 500, "internal_error", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{ErrorHandler: Handler})
			app.Get("/", func(c *fiber.Ctx) error { return tc.err })
			resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
			if body["code"] != tc.wantCode {
				t.Fatalf("code: got %v, want %v", body["code"], tc.wantCode)
			}
			for _, k := range []string{"title", "details"} {
				if s, _ := body[k].(string); s == "" {
					t.Fatalf("expected non-empty %q in %v", k, body)
				}
			}
			for k, v := range tc.wantExtra {
				got, _ := json.Marshal(body[k])
				want, _ := json.Marshal(v)
				if string(got) != string(want) {
					t.Fatalf("extra %q: got %s, want %s", k, got, want)
				}
			}
		})
	}
}

func TestHandlerPassthrough(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: Handler})
	app.Get("/", func(c *fiber.Ctx) error { return fiber.ErrTeapot })
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 418 {
		t.Fatalf("expected Fiber default passthrough (418), got %d", resp.StatusCode)
	}
}
