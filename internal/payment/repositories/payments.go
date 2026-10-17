package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"paymentgateway/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type PaymentDatabaseRepository struct{ db *sql.DB }

func NewPaymentDatabaseRepository(db *sql.DB) *PaymentDatabaseRepository {
	return &PaymentDatabaseRepository{db}
}

func (r *PaymentDatabaseRepository) InsertPending(ctx context.Context, p *domain.Payment) error {
	p.ID = uuid.NewString()
	p.Status = domain.StatusPending

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO payments (id, merchant_id, amount, idempotency_key, status) VALUES ($1,$2,$3,$4,$5)`,
		p.ID, p.MerchantID, p.Amount, p.IdempotencyKey, p.Status)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrConflict
		}
		return err
	}

	return r.db.QueryRowContext(ctx,
		`SELECT created_at FROM payments WHERE id=$1`, p.ID).Scan(&p.CreatedAt)
}

func (r *PaymentDatabaseRepository) FindByIdem(ctx context.Context, merchantID, key string) (*domain.Payment, error) {
	var p domain.Payment
	var status string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, amount, idempotency_key, status, created_at FROM payments WHERE merchant_id=$1 AND idempotency_key=$2`,
		merchantID, key).Scan(&p.ID, &p.MerchantID, &p.Amount, &p.IdempotencyKey, &status, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	p.Status = domain.Status(status)
	return &p, err
}

func (r *PaymentDatabaseRepository) MarkFailed(ctx context.Context, p *domain.Payment) error {
	p.Status = domain.StatusFailed
	result, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET status = 'failed'
		WHERE id = $1
		  AND status <> 'failed'
	`, p.ID)
	if err != nil {
		return fmt.Errorf("mark ledger entry %q as failed: %w", p.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows for ledger entry %q: %w", p.ID, err)
	}

	if rows == 0 {
		return fmt.Errorf("ledger entry %q not found or already failed", p.ID)
	}

	return nil
}

func (r *PaymentDatabaseRepository) MarkAuthorized(ctx context.Context, p *domain.Payment) error {
	p.Status = domain.StatusAuthorized
	result, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET status = 'authorized'
		WHERE id = $1
		  AND status = 'pending'
	`, p.ID)
	if err != nil {
		return fmt.Errorf("mark payment %q as authorized: %w", p.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows for payment %q: %w", p.ID, err)
	}

	if rows == 0 {
		return fmt.Errorf("payment %q not found or not pending", p.ID)
	}

	return nil
}

func (r *PaymentDatabaseRepository) MarkCaptured(ctx context.Context, p *domain.Payment) error {
	p.Status = domain.StatusCaptured
	result, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET status = 'captured'
		WHERE id = $1
		  AND status = 'authorized'
	`, p.ID)
	if err != nil {
		return fmt.Errorf("mark payment %q as captured: %w", p.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows for payment %q: %w", p.ID, err)
	}

	if rows == 0 {
		return fmt.Errorf("payment %q not found or not authorized", p.ID)
	}

	return nil
}

////////

func (r *PaymentDatabaseRepository) New(ctx context.Context, p *domain.PaymentPending) error {
	p.ID = uuid.NewString()
	p.Status = domain.StatusPending

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO payments (id, merchant_id, amount, idempotency_key, status) VALUES ($1,$2,$3,$4,$5)`,
		p.ID, p.MerchantID, p.Amount, p.IdempotencyKey, p.Status)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrConflict
		}
		return err
	}

	return r.db.QueryRowContext(ctx,
		`SELECT created_at FROM payments WHERE id=$1`, p.ID).Scan(&p.CreatedAt)
}

func (r *PaymentDatabaseRepository) Authorized(ctx context.Context, p domain.PaymentPending) (domain.PaymentAuthenticated, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET status = 'authorized'
		WHERE id = $1
		  AND status = 'pending'
	`, p.ID)
	if err != nil {
		return domain.PaymentAuthenticated{}, fmt.Errorf("mark payment %q as authorized: %w", p.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return domain.PaymentAuthenticated{}, fmt.Errorf("get affected rows for payment %q: %w", p.ID, err)
	}

	if rows == 0 {
		return domain.PaymentAuthenticated{}, fmt.Errorf("payment %q not found or not pending", p.ID)
	}

	return domain.PaymentAuthenticated{PaymentPending: p}, nil
}

func (r *PaymentDatabaseRepository) Captured(ctx context.Context, p domain.PaymentAuthenticated) (domain.PaymentCaptured, error) {
	return domain.PaymentCaptured{}, nil
}

func (r *PaymentDatabaseRepository) Failed(ctx context.Context, paymentId string, err error) (domain.PaymentFailed, error) {
	return domain.PaymentFailed{}, nil
}
