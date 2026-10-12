package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"paymentgateway/pkg/tracing"
)

func offlineContainer(t *testing.T) *Container {
	t.Helper()
	appDB, _ := sql.Open("pgx", "postgres://u:p@127.0.0.1:1/app?sslmode=disable")
	ledgerDB, _ := sql.Open("pgx", "postgres://u:p@127.0.0.1:1/ledger?sslmode=disable")
	tp := tracing.Init()
	return &Container{AppDB: appDB, LedgerDB: ledgerDB, Redis: redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}), Tracer: tp}
}

// TestShutdownClosesPools verifies pools close and double shutdown is safe.
func TestShutdownClosesPools(t *testing.T) {
	c := offlineContainer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if err := c.AppDB.PingContext(ctx); err == nil {
		t.Fatal("app pool still open after shutdown")
	}
	if err := c.Shutdown(ctx); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
}
