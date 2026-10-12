package payment

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestEnqueueReconciliationRoundTrip verifies an in-doubt payment is
// recorded and re-enqueue is a no-op. Needs a migrated app DB; skips
// otherwise (unit suites stay hermetic).
func TestEnqueueReconciliationRoundTrip(t *testing.T) {
	dsn := os.Getenv("APP_DSN")
	if dsn == "" {
		t.Skip("APP_DSN not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("app DB unreachable: %v", err)
	}

	mid := "11111111-1111-1111-1111-111111111111"
	if _, err := db.ExecContext(ctx,
		`INSERT INTO merchants (id, name, email) VALUES ($1,'t','t@t.t') ON CONFLICT DO NOTHING`, mid); err != nil {
		t.Fatal(err)
	}
	p := &Payment{MerchantID: mid, Amount: 100, IdempotencyKey: "k-outbox-1"}
	r := NewAppRepository(db)
	if err := r.InsertPending(ctx, p); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("status query inconclusive")
	if err := r.EnqueueReconciliation(ctx, p.ID, p.IdempotencyKey, "call-api", cause); err != nil {
		t.Fatal(err)
	}
	if err := r.EnqueueReconciliation(ctx, p.ID, p.IdempotencyKey, "call-api", cause); err != nil {
		t.Fatalf("re-enqueue failed: %v", err)
	}

	var gotKey, gotStep, gotErr string
	var attempts int
	err = db.QueryRowContext(ctx,
		`SELECT idempotency_key, step, error, attempts FROM reconciliation_outbox WHERE payment_id=$1`,
		p.ID).Scan(&gotKey, &gotStep, &gotErr, &attempts)
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "k-outbox-1" || gotStep != "call-api" || gotErr != cause.Error() || attempts != 0 {
		t.Fatalf("bad outbox row: %q %q %q %d", gotKey, gotStep, gotErr, attempts)
	}
}
