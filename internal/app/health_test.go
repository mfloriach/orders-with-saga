package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"paymentgateway/internal/config"

	"github.com/gofiber/fiber/v2"
)

func get(t *testing.T, f *fiber.App, path string) (int, map[string]any) {
	t.Helper()
	resp, err := f.Test(httptest.NewRequest("GET", path, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

func okCheck(context.Context) error { return nil }

// TestReadyAllUp verifies 200 with per-dep ok status.
func TestReadyAllUp(t *testing.T) {
	f := fiber.New()
	f.Get("/ready", readyHandler(map[string]func(ctx context.Context) error{
		"app_db": okCheck, "ledger_db": okCheck, "redis": okCheck,
	}))
	status, body := get(t, f, "/ready")
	if status != 200 || body["ok"] != true {
		t.Fatalf("got %d %v", status, body)
	}
}

// TestReadyNamesFailures verifies 503 naming the down dependency.
func TestReadyNamesFailures(t *testing.T) {
	f := fiber.New()
	f.Get("/ready", readyHandler(map[string]func(ctx context.Context) error{
		"app_db": okCheck,
		"redis":  func(context.Context) error { return errors.New("refused") },
	}))
	status, body := get(t, f, "/ready")
	if status != 503 || body["ok"] != false {
		t.Fatalf("got %d %v", status, body)
	}
	failed, _ := body["failed"].([]any)
	if len(failed) != 1 || failed[0] != "redis" {
		t.Fatalf("bad failed list: %v", body)
	}
}

// TestHealthUnchanged pins the liveness contract.
func TestHealthUnchanged(t *testing.T) {
	f := fiber.New()
	f.Get("/health", Health)
	status, body := get(t, f, "/health")
	if status != 200 || body["ok"] != true || len(body) != 1 {
		t.Fatalf("got %d %v", status, body)
	}
}

// TestMastercardNetworkWiring verifies wiring builds without dialing.
// Unconfigured base still builds; the failure surfaces at call time
// (fail-closed downstream), never at boot.
func TestMastercardNetworkWiring(t *testing.T) {
	p, err := cardNetwork(config.Config{})
	if err != nil || p == nil {
		t.Fatalf("unset: got %v %v", p, err)
	}
	p, err = cardNetwork(config.Config{MCBaseURL: "http://mc:8080", MCAPIKey: "k"})
	if err != nil || p == nil {
		t.Fatalf("got %v %v", p, err)
	}
}
