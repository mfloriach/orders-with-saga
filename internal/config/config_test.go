package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range []string{"PORT", "APP_DSN", "LEDGER_DSN", "REDIS_ADDR", "RATE_LIMIT", "RATE_WINDOW_SEC",
		"PG_MAX_OPEN", "PG_MAX_IDLE", "PG_MAX_LIFETIME_SEC", "PG_MAX_IDLE_SEC", "PG_CONNECT_TIMEOUT_SEC",
		"REDIS_POOL_SIZE", "REDIS_DIAL_TIMEOUT_SEC", "REDIS_READ_TIMEOUT_SEC", "REDIS_WRITE_TIMEOUT_SEC",
		"MC_BASE_URL", "MC_API_KEY", "MC_TIMEOUT_SEC", "MC_MAX_ATTEMPTS", "MC_BASE_BACKOFF_MS", "MC_MAX_BACKOFF_MS", "MC_OVERALL_TIMEOUT_SEC"} {
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
		"APP_DSN":    "postgres://app:app@localhost:5432/app?sslmode=disable",
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
		{"zero pg max open", merge(valid, map[string]string{"PG_MAX_OPEN": "0"}), "PGMaxOpen"},
		{"negative pg timeout", merge(valid, map[string]string{"PG_CONNECT_TIMEOUT_SEC": "-1"}), "PGConnectTimeout"},
		{"negative redis pool", merge(valid, map[string]string{"REDIS_POOL_SIZE": "-1"}), "RedisPoolSize"},
		{"zero mc timeout", merge(valid, map[string]string{"MC_TIMEOUT_SEC": "0"}), "MCTimeout"},
		{"zero mc attempts", merge(valid, map[string]string{"MC_MAX_ATTEMPTS": "0"}), "MCMaxAttempts"},
		{"zero mc overall timeout", merge(valid, map[string]string{"MC_OVERALL_TIMEOUT_SEC": "0"}), "MCOverallTimeout"},
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

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	content := "# comment\nAPP_DSN=postgres://file/app\nLEDGER_DSN=postgres://file/ledger\nREDIS_ADDR=file-redis:6379\nPORT=9001\n\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	withEnv(t, map[string]string{})
	cfg, err := load(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.AppDSN != "postgres://file/app" || cfg.Port != 9001 {
		t.Fatalf("file values not loaded: %+v", cfg)
	}
}

func TestEnvWinsOverFile(t *testing.T) {
	dir := t.TempDir()
	content := "APP_DSN=postgres://file/app\nLEDGER_DSN=postgres://file/ledger\nREDIS_ADDR=file-redis:6379\nPORT=9001\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	withEnv(t, map[string]string{"PORT": "9002"})
	cfg, err := load(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Port != 9002 {
		t.Fatalf("expected env PORT=9002 to win, got %d", cfg.Port)
	}
}
