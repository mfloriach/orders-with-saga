package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrConflict = errors.New("idempotency_conflict")

type PaymentRepository interface {
	InsertPending(ctx context.Context, p *Payment) error
	FindByIdem(ctx context.Context, merchantID, key string) (*Payment, error)
	MarkFailed(ctx context.Context, id string) error
	MarkAuthorized(ctx context.Context, id string) error
	MarkCaptured(ctx context.Context, id string) error
	EnqueueReconciliation(ctx context.Context, paymentID, idemKey, step string, cause error) error
}

type AppRepository struct{ db *sql.DB }

func NewAppRepository(db *sql.DB) *AppRepository { return &AppRepository{db} }

func (r *AppRepository) InsertPending(ctx context.Context, p *Payment) error {
	p.ID = uuid.NewString()
	p.Status = "pending"
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO payments (id, merchant_id, amount, idempotency_key, status) VALUES ($1,$2,$3,$4,$5)`,
		p.ID, p.MerchantID, p.Amount, p.IdempotencyKey, p.Status)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return err
	}
	return r.db.QueryRowContext(ctx,
		`SELECT created_at FROM payments WHERE id=$1`, p.ID).Scan(&p.CreatedAt)
}

func (r *AppRepository) FindByIdem(ctx context.Context, merchantID, key string) (*Payment, error) {
	var p Payment
	err := r.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, amount, idempotency_key, status, created_at FROM payments WHERE merchant_id=$1 AND idempotency_key=$2`,
		merchantID, key).Scan(&p.ID, &p.MerchantID, &p.Amount, &p.IdempotencyKey, &p.Status, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

// EnqueueReconciliation records an in-doubt payment for later batch
// processing. Re-enqueue of the same payment is a no-op.
func (r *AppRepository) EnqueueReconciliation(ctx context.Context, paymentID, idemKey, step string, cause error) error {
	msg := ""
	if cause != nil {
		msg = cause.Error()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO reconciliation_outbox (payment_id, idempotency_key, step, error)
		 VALUES ($1,$2,$3,$4) ON CONFLICT (payment_id) DO NOTHING`,
		paymentID, idemKey, step, msg)
	return err
}

func (r *AppRepository) MarkFailed(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE payments
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

// MarkAuthorized moves a pending payment to authorized. The pending
// precondition makes double-marking fail loudly instead of rewriting
// history; callers check the transition first for a decision-time error.
func (r *AppRepository) MarkAuthorized(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET status = 'authorized'
		WHERE id = $1
		  AND status = 'pending'
	`, id)
	if err != nil {
		return fmt.Errorf("mark payment %q as authorized: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows for payment %q: %w", id, err)
	}

	if rows == 0 {
		return fmt.Errorf("payment %q not found or not pending", id)
	}

	return nil
}

func (r *AppRepository) MarkCaptured(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET status = 'captured'
		WHERE id = $1
		  AND status = 'authorized'
	`, id)
	if err != nil {
		return fmt.Errorf("mark payment %q as captured: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows for payment %q: %w", id, err)
	}

	if rows == 0 {
		return fmt.Errorf("payment %q not found or not authorized", id)
	}

	return nil
}
