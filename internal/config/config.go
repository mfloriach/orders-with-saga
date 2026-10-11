package config

import "os"

type Config struct {
	Port       string
	AppDSN     string
	LedgerDSN  string
	RedisAddr  string
	RateLimit  int
	RateWindow int // seconds
}

func Load() Config {
	return Config{
		Port:       env("PORT", "8080"),
		AppDSN:     env("APP_DSN", "postgres://app:app@localhost:5432/app?sslmode=disable"),
		LedgerDSN:  env("LEDGER_DSN", "postgres://ledger:ledger@localhost:5433/ledger?sslmode=disable"),
		RedisAddr:  env("REDIS_ADDR", "localhost:6379"),
		RateLimit:  100,
		RateWindow: 60,
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
