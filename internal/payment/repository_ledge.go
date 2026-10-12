package payment

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type LedgerRepository struct{ db *sql.DB }

func NewLedgerRepository(db *sql.DB) *LedgerRepository { return &LedgerRepository{db} }

func (r *LedgerRepository) Insert(ctx context.Context, paymentID string, amount int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ledger_entries (id, payment_id, amount) VALUES ($1,$2,$3)`,
		uuid.NewString(), paymentID, amount)
	return err
}

func (r *LedgerRepository) MarkFailed(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE ledger_entries
		SET status = 'failed'
		WHERE id = $1
		  AND status <> 'failed'
	`, id)
	if err != nil {
		return fmt.Errorf("mark ledger entry %q as failed: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows for ledger entry %q: %w", id, err)
	}

	if rows == 0 {
		return fmt.Errorf("ledger entry %q not found or already failed", id)
	}

	return nil
}
