package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"paymentgateway/internal/config"
	"paymentgateway/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type payment struct {
	ID                string
	MerchantID        string
	Amount            int64
	Coin              string
	IdempotencyKey    string
	Status            domain.PaymentStatus
	CreatedAt         time.Time
	AuthenticationKey string
	PaidAt            time.Time
	Reason            string
	RefundAt          time.Time
}

type paymentDatabaseRepository struct{ db *sql.DB }

func NewPaymentDatabaseRepository(db *sql.DB) *paymentDatabaseRepository {
	return &paymentDatabaseRepository{db}
}

func (r *paymentDatabaseRepository) New(ctx context.Context, p *domain.PaymentPending) error {
	p.ID = uuid.NewString()
	p.Status = domain.PaymentStatusPending

	_, err := config.Executor(ctx, r.db).ExecContext(ctx,
		`INSERT INTO payments (id, merchant_id, amount, coin, idempotency_key, status) VALUES ($1,$2,$3,$4,$5,$6)`,
		p.ID, p.MerchantID, p.Price.Amount, p.Price.Coin, p.IdempotencyKey, p.Status)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrConflict
		}
		return err
	}

	return config.Executor(ctx, r.db).QueryRowContext(ctx,
		`SELECT created_at FROM payments WHERE id=$1`, p.ID).Scan(&p.CreatedAt)
}

func (r *paymentDatabaseRepository) Authorized(ctx context.Context, p domain.PaymentPending, authorizationID string) (domain.PaymentAuthenticated, error) {
	result, err := config.Executor(ctx, r.db).ExecContext(ctx, `
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

	return domain.PaymentAuthenticated{PaymentPending: p, AuthenticationKey: authorizationID}, nil
}

func (r *paymentDatabaseRepository) Captured(ctx context.Context, p domain.PaymentAuthenticated) (domain.PaymentCaptured, error) {
	result, err := config.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE payments
		SET status = 'captured'
		WHERE id = $1
		  AND status = 'authorized'
	`, p.ID)
	if err != nil {
		return domain.PaymentCaptured{}, fmt.Errorf("mark payment %q as captured: %w", p.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return domain.PaymentCaptured{}, fmt.Errorf("get affected rows for payment %q: %w", p.ID, err)
	}

	if rows == 0 {
		return domain.PaymentCaptured{}, fmt.Errorf("payment %q not found or not authorized", p.ID)
	}

	return domain.PaymentCaptured{
		PaymentAuthenticated: p,
		PaidAt:               time.Now(),
	}, nil
}

func (r *paymentDatabaseRepository) Failed(ctx context.Context, paymentId string, err error) (domain.PaymentFailed, error) {
	result, execErr := config.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE payments
		SET status = 'failed'
		WHERE id = $1
		  AND status IN ('pending', 'authorized')
	`, paymentId)
	if execErr != nil {
		return domain.PaymentFailed{}, fmt.Errorf("mark payment %q as failed: %w", paymentId, execErr)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return domain.PaymentFailed{}, fmt.Errorf("get affected rows for payment %q: %w", paymentId, rowsErr)
	}

	if rows == 0 {
		return domain.PaymentFailed{}, fmt.Errorf("payment %q not found or not in a failable state", paymentId)
	}

	return domain.PaymentFailed{
		Reason: err.Error(),
	}, nil
}

func (r *paymentDatabaseRepository) IsIdempotent(ctx context.Context, merchantID, key string) (string, error) {
	var id string

	err := config.Executor(ctx, r.db).QueryRowContext(ctx, `
        SELECT id
        FROM payments
        WHERE merchant_id = $1
          AND idempotency_key = $2
    `, merchantID, key).Scan(&id)

	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("check payment idempotency: %w", err)
	}

	return id, nil
}
