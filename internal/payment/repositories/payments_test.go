package repositories

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"paymentgateway/internal/domain"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// openTestDB opens the migrated app DB or skips when unavailable, keeping
// unit suites hermetic.
func openTestDB(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv("APP_DSN")
	if dsn == "" {
		t.Skip("APP_DSN not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("app DB unreachable: %v", err)
	}
	return db, ctx
}

func seedPending(t *testing.T, ctx context.Context, db *sql.DB, key string) *domain.Payment {
	t.Helper()
	mid := "11111111-1111-1111-1111-111111111111"
	if _, err := db.ExecContext(ctx,
		`INSERT INTO merchants (id, name, email) VALUES ($1,'t','t@t.t') ON CONFLICT DO NOTHING`, mid); err != nil {
		t.Fatal(err)
	}
	p := &domain.Payment{MerchantID: mid, Amount: 100, IdempotencyKey: key}
	if err := NewPaymentDatabaseRepository(db).InsertPending(ctx, p); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestMarkAuthorizedRoundTrip verifies pending→authorized persists and a
// second mark fails loudly instead of rewriting history.
func TestMarkAuthorizedRoundTrip(t *testing.T) {
	db, ctx := openTestDB(t)
	r := NewPaymentDatabaseRepository(db)
	p := seedPending(t, ctx, db, "k-auth-1")

	if err := r.MarkAuthorized(ctx, p); err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.StatusAuthorized {
		t.Fatalf("p.Status=%q, want authorized", p.Status)
	}
	var status string
	if err := db.QueryRowContext(ctx,
		`SELECT status FROM payments WHERE id=$1`, p.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != string(domain.StatusAuthorized) {
		t.Fatalf("status=%q, want authorized", status)
	}
	if err := r.MarkAuthorized(ctx, p); err == nil {
		t.Fatal("expected error on double mark")
	}
}

// TestEnqueueReconciliationRoundTrip verifies an in-doubt payment is
// recorded and re-enqueue is a no-op. Needs a migrated app DB; skips
// otherwise (unit suites stay hermetic).
func TestEnqueueReconciliationRoundTrip(t *testing.T) {
	db, ctx := openTestDB(t)

	mid := "11111111-1111-1111-1111-111111111111"
	if _, err := db.ExecContext(ctx,
		`INSERT INTO merchants (id, name, email) VALUES ($1,'t','t@t.t') ON CONFLICT DO NOTHING`, mid); err != nil {
		t.Fatal(err)
	}
	p := &domain.Payment{MerchantID: mid, Amount: 100, IdempotencyKey: "k-outbox-1"}
	r := NewPaymentDatabaseRepository(db)
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
	err := db.QueryRowContext(ctx,
		`SELECT idempotency_key, step, error, attempts FROM reconciliation_outbox WHERE payment_id=$1`,
		p.ID).Scan(&gotKey, &gotStep, &gotErr, &attempts)
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "k-outbox-1" || gotStep != "call-api" || gotErr != cause.Error() || attempts != 0 {
		t.Fatalf("bad outbox row: %q %q %q %d", gotKey, gotStep, gotErr, attempts)
	}
}
