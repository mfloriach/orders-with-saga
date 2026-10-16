package merchant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"paymentgateway/internal/domain"
	"paymentgateway/pkg/apperr"

	"github.com/gofiber/fiber/v2"
)

type stubService struct {
	register func(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error)
}

func (s stubService) Register(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error) {
	return s.register(ctx, name, email, webhook)
}

func TestCreateErrors(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		stub       func(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error)
		wantStatus int
		wantCode   string
	}{
		{"invalid json", `{bad`, nil, 400, "validation_error"},
		{"validation failure", `{"name":"","email":"bad"}`, nil, 400, "validation_error"},
		{
			"email taken",
			`{"name":"Acme","email":"a@b.com"}`,
			func(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error) {
				return nil, ErrEmailTaken
			},
			409, "email_taken",
		},
		{
			"internal error",
			`{"name":"Acme","email":"a@b.com"}`,
			func(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error) {
				return nil, errors.New("boom")
			},
			500, "internal_error",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(stubService{register: tc.stub})
			app := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})
			app.Post("/merchants", h.Create)
			req := httptest.NewRequest("POST", "/merchants", strings.NewReader(tc.body))
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
		})
	}
}

func TestCreateSuccess(t *testing.T) {
	hook := "https://acme.example/hook"
	h := NewHandler(stubService{register: func(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error) {
		return &domain.Merchant{ID: "m-1", Name: name, Email: email, WebhookURL: &hook}, nil
	}})
	app := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})
	app.Post("/merchants", h.Create)
	req := httptest.NewRequest("POST", "/merchants", strings.NewReader(`{"name":"Acme","email":"a@b.com"}`))
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
