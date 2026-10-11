package payment

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrConflict = errors.New("idempotency_conflict")

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

func (r *AppRepository) MarkFailed(ctx context.Context, id string) {
	r.db.ExecContext(ctx, `UPDATE payments SET status='failed' WHERE id=$1`, id)
}

type LedgerRepository struct{ db *sql.DB }

func NewLedgerRepository(db *sql.DB) *LedgerRepository { return &LedgerRepository{db} }

func (r *LedgerRepository) Insert(ctx context.Context, paymentID string, amount int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ledger_entries (id, payment_id, amount) VALUES ($1,$2,$3)`,
		uuid.NewString(), paymentID, amount)
	return err
}
