package payment

import (
	"context"
	"database/sql"
	"os"
	"testing"

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

func seedPending(t *testing.T, ctx context.Context, db *sql.DB, key string) *Payment {
	t.Helper()
	mid := "11111111-1111-1111-1111-111111111111"
	if _, err := db.ExecContext(ctx,
		`INSERT INTO merchants (id, name, email) VALUES ($1,'t','t@t.t') ON CONFLICT DO NOTHING`, mid); err != nil {
		t.Fatal(err)
	}
	p := &Payment{MerchantID: mid, Amount: 100, IdempotencyKey: key}
	if err := NewAppRepository(db).InsertPending(ctx, p); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestMarkAuthorizedRoundTrip verifies pending→authorized persists and a
// second mark fails loudly instead of rewriting history.
func TestMarkAuthorizedRoundTrip(t *testing.T) {
	db, ctx := openTestDB(t)
	r := NewAppRepository(db)
	p := seedPending(t, ctx, db, "k-auth-1")

	if err := r.MarkAuthorized(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.QueryRowContext(ctx,
		`SELECT status FROM payments WHERE id=$1`, p.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != StatusAuthorized {
		t.Fatalf("status=%q, want authorized", status)
	}
	if err := r.MarkAuthorized(ctx, p.ID); err == nil {
		t.Fatal("expected error on double mark")
	}
}
