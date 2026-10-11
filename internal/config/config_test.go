package config

import (
	"os"
	"strings"
	"testing"
)

func withEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range []string{"PORT", "APP_DSN", "LEDGER_DSN", "REDIS_ADDR", "RATE_LIMIT", "RATE_WINDOW_SEC"} {
		v, ok := env[k]
		if !ok || v == "" {
			os.Unsetenv(k)
		} else {
			t.Setenv(k, v)
		}
	}
}

func TestLoad(t *testing.T) {
	valid := map[string]string{
		"APP_DSN": "postgres://app:app@localhost:5432/app?sslmode=disable",
		"LEDGER_DSN": "postgres://ledger:ledger@localhost:5433/ledger?sslmode=disable",
		"REDIS_ADDR": "localhost:6379",
	}
	cases := []struct {
		name    string
		env     map[string]string
		wantErr string // "" means no error; otherwise substring of error
	}{
		{"valid minimal", valid, ""},
		{"valid with overrides", merge(valid, map[string]string{"PORT": "9000", "RATE_LIMIT": "50"}), ""},
		{"missing app DSN", merge(valid, map[string]string{"APP_DSN": ""}), "AppDSN"},
		{"bad port", merge(valid, map[string]string{"PORT": "abc"}), "PORT"},
		{"zero rate limit", merge(valid, map[string]string{"RATE_LIMIT": "0"}), "RateLimit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t, tc.env)
			cfg, err := Load()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if cfg.RateLimit != 100 && tc.env["RATE_LIMIT"] == "" {
					t.Fatalf("expected default rate limit 100, got %d", cfg.RateLimit)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error naming %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func merge(base, over map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}
