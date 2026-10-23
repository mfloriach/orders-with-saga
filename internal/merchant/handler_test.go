package merchant

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"paymentgateway/internal/domain"
	"paymentgateway/pkg/apperr"

	"github.com/gofiber/fiber/v2"
)

type stubService struct {
	register func(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error)
	list     func(ctx context.Context, limit, offset int) ([]domain.Merchant, int, error)
	get      func(ctx context.Context, id string) (*domain.Merchant, error)
	update   func(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error)
	delete   func(ctx context.Context, id string) error
}

func (s stubService) Register(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error) {
	return s.register(ctx, name, email, webhook)
}

func (s stubService) List(ctx context.Context, limit, offset int) ([]domain.Merchant, int, error) {
	return s.list(ctx, limit, offset)
}

func (s stubService) Get(ctx context.Context, id string) (*domain.Merchant, error) {
	return s.get(ctx, id)
}

func (s stubService) Update(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error) {
	return s.update(ctx, id, name, email, webhook)
}

func (s stubService) Delete(ctx context.Context, id string) error {
	return s.delete(ctx, id)
}

func newTestApp(h *Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})
	app.Post("/merchants", h.Create)
	app.Get("/merchants", h.List)
	app.Get("/merchants/:id", h.Get)
	app.Patch("/merchants/:id", h.Update)
	app.Delete("/merchants/:id", h.Delete)
	return app
}

func request(t *testing.T, app *fiber.App, method, path, body string) (int, map[string]any) {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		return resp.StatusCode, nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode response %q: %v", raw, err)
	}
	return resp.StatusCode, decoded
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
			status, body := request(t, newTestApp(h), "POST", "/merchants", tc.body)
			if status != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", status, tc.wantStatus)
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

	status, body := request(t, newTestApp(h), "POST", "/merchants", `{"name":"Acme","email":"a@b.com"}`)
	if status != 201 {
		t.Fatalf("status: got %d, want 201", status)
	}
	if body["id"] != "m-1" {
		t.Fatalf("id: got %v, want m-1", body["id"])
	}
}

func TestListDefaultsAndEnvelope(t *testing.T) {
	var gotLimit, gotOffset int
	h := NewHandler(stubService{list: func(ctx context.Context, limit, offset int) ([]domain.Merchant, int, error) {
		gotLimit, gotOffset = limit, offset
		return nil, 0, nil
	}})

	status, body := request(t, newTestApp(h), "GET", "/merchants", "")
	if status != 200 {
		t.Fatalf("status: got %d, want 200", status)
	}
	if gotLimit != defaultLimit || gotOffset != 0 {
		t.Fatalf("list args: got limit=%d offset=%d, want %d/0", gotLimit, gotOffset, defaultLimit)
	}

	data, ok := body["data"].([]any)
	if !ok || len(data) != 0 {
		t.Fatalf("data: got %#v, want empty array", body["data"])
	}
	pagination := body["pagination"].(map[string]any)
	if pagination["page"] != float64(1) || pagination["limit"] != float64(defaultLimit) ||
		pagination["total_items"] != float64(0) || pagination["total_pages"] != float64(0) {
		t.Fatalf("pagination: got %#v", pagination)
	}
}

func TestListAcceptsMaxPagination(t *testing.T) {
	var gotLimit, gotOffset int
	h := NewHandler(stubService{list: func(ctx context.Context, limit, offset int) ([]domain.Merchant, int, error) {
		gotLimit, gotOffset = limit, offset
		return []domain.Merchant{}, 250, nil
	}})

	status, body := request(t, newTestApp(h), "GET", "/merchants?page=100&limit=50", "")
	if status != 200 {
		t.Fatalf("status: got %d, want 200", status)
	}
	if gotLimit != maxLimit || gotOffset != 99*maxLimit {
		t.Fatalf("list args: got limit=%d offset=%d, want %d/%d", gotLimit, gotOffset, maxLimit, 99*maxLimit)
	}
	if body["pagination"].(map[string]any)["total_pages"] != float64(5) {
		t.Fatalf("total_pages: got %v, want 5", body["pagination"].(map[string]any)["total_pages"])
	}
}

func TestListRejectsOutOfBoundsPagination(t *testing.T) {
	cases := []string{
		"/merchants?page=-1",
		"/merchants?page=101",
		"/merchants?limit=-1",
		"/merchants?limit=51",
	}

	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			h := NewHandler(stubService{list: func(ctx context.Context, limit, offset int) ([]domain.Merchant, int, error) {
				t.Fatal("service should not be called")
				return nil, 0, nil
			}})

			status, body := request(t, newTestApp(h), "GET", path, "")
			if status != 400 || body["code"] != "validation_error" {
				t.Fatalf("got status=%d code=%v, want 400 validation_error", status, body["code"])
			}
		})
	}
}

func TestGet(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		h := NewHandler(stubService{get: func(ctx context.Context, id string) (*domain.Merchant, error) {
			return &domain.Merchant{ID: id, Name: "Acme", Email: "a@b.com"}, nil
		}})
		status, body := request(t, newTestApp(h), "GET", "/merchants/m-1", "")
		if status != 200 || body["id"] != "m-1" {
			t.Fatalf("got status=%d body=%#v", status, body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		h := NewHandler(stubService{get: func(ctx context.Context, id string) (*domain.Merchant, error) {
			return nil, ErrNotFound
		}})
		status, body := request(t, newTestApp(h), "GET", "/merchants/m-1", "")
		if status != 404 || body["code"] != "merchant_not_found" {
			t.Fatalf("got status=%d code=%v", status, body["code"])
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Run("partial body keeps omitted fields", func(t *testing.T) {
		var gotName, gotEmail, gotWebhook *string
		h := NewHandler(stubService{update: func(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error) {
			gotName, gotEmail, gotWebhook = name, email, webhook
			return &domain.Merchant{ID: id, Name: *name, Email: "a@b.com"}, nil
		}})

		status, _ := request(t, newTestApp(h), "PATCH", "/merchants/m-1", `{"name":"New Name"}`)
		if status != 200 {
			t.Fatalf("status: got %d, want 200", status)
		}
		if gotName == nil || *gotName != "New Name" {
			t.Fatalf("name: got %v, want New Name", gotName)
		}
		if gotEmail != nil || gotWebhook != nil {
			t.Fatalf("omitted fields: got email=%v webhook=%v, want nil", gotEmail, gotWebhook)
		}
	})

	t.Run("empty body is a no-op", func(t *testing.T) {
		h := NewHandler(stubService{update: func(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error) {
			if name != nil || email != nil || webhook != nil {
				t.Fatalf("expected no updates, got %v %v %v", name, email, webhook)
			}
			return &domain.Merchant{ID: id}, nil
		}})
		status, _ := request(t, newTestApp(h), "PATCH", "/merchants/m-1", "")
		if status != 200 {
			t.Fatalf("status: got %d, want 200", status)
		}
	})

	t.Run("empty webhook clears it", func(t *testing.T) {
		var gotWebhook *string
		h := NewHandler(stubService{update: func(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error) {
			gotWebhook = webhook
			return &domain.Merchant{ID: id}, nil
		}})
		status, _ := request(t, newTestApp(h), "PATCH", "/merchants/m-1", `{"webhook_url":""}`)
		if status != 200 {
			t.Fatalf("status: got %d, want 200", status)
		}
		if gotWebhook == nil || *gotWebhook != "" {
			t.Fatalf("webhook: got %v, want pointer to empty string", gotWebhook)
		}
	})

	t.Run("email conflict", func(t *testing.T) {
		h := NewHandler(stubService{update: func(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error) {
			return nil, ErrEmailTaken
		}})
		status, body := request(t, newTestApp(h), "PATCH", "/merchants/m-1", `{"email":"taken@b.com"}`)
		if status != 409 || body["code"] != "email_taken" {
			t.Fatalf("got status=%d code=%v", status, body["code"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		h := NewHandler(stubService{update: func(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error) {
			return nil, ErrNotFound
		}})
		status, body := request(t, newTestApp(h), "PATCH", "/merchants/m-1", `{"name":"Acme"}`)
		if status != 404 || body["code"] != "merchant_not_found" {
			t.Fatalf("got status=%d code=%v", status, body["code"])
		}
	})
}

func TestDelete(t *testing.T) {
	t.Run("deleted", func(t *testing.T) {
		var gotID string
		h := NewHandler(stubService{delete: func(ctx context.Context, id string) error {
			gotID = id
			return nil
		}})
		status, _ := request(t, newTestApp(h), "DELETE", "/merchants/m-1", "")
		if status != 204 {
			t.Fatalf("status: got %d, want 204", status)
		}
		if gotID != "m-1" {
			t.Fatalf("id: got %q, want m-1", gotID)
		}
	})

	t.Run("already absent is idempotent", func(t *testing.T) {
		h := NewHandler(stubService{delete: func(ctx context.Context, id string) error {
			return ErrNotFound
		}})
		status, _ := request(t, newTestApp(h), "DELETE", "/merchants/m-1", "")
		if status != 204 {
			t.Fatalf("status: got %d, want 204", status)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		h := NewHandler(stubService{delete: func(ctx context.Context, id string) error {
			return errors.New("boom")
		}})
		status, body := request(t, newTestApp(h), "DELETE", "/merchants/m-1", "")
		if status != 500 || body["code"] != "internal_error" {
			t.Fatalf("got status=%d code=%v", status, body["code"])
		}
	})
}
