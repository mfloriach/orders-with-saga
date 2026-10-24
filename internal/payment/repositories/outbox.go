package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"paymentgateway/internal/config"
	"paymentgateway/internal/domain"

	"github.com/google/uuid"
)

type outboxPayment struct {
	ID          string
	PaymentID   string
	MerchantID  string
	Status      domain.OutboxStatus
	Attempts    int64
	LastError   string
	CreatedAt   time.Time
	ProcessedAt sql.NullTime
}

type paymentOutbox struct{ db *sql.DB }

func NewPaymentOutbox(db *sql.DB) *paymentOutbox {
	return &paymentOutbox{db}
}

func (r *paymentOutbox) Store(ctx context.Context, p domain.PaymentAuthenticated) error {
	_, err := config.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO outbox_payments (id, payment_id, merchant_id)
		VALUES ($1, $2, $3)
	`, uuid.NewString(), p.ID, p.MerchantID)
	if err != nil {
		return fmt.Errorf("enqueue captured payment %q for processing: %w", p.ID, err)
	}

	return nil
}

func (r *paymentOutbox) GetByStatus(ctx context.Context) ([]*domain.OutboxPayment, error) {
	rows, err := config.Executor(ctx, r.db).QueryContext(ctx, `
		SELECT o.id, o.payment_id, o.merchant_id, o.status, o.attempts, o.last_error, o.created_at, o.processed_at
		FROM outbox_payments o
		WHERE o.status = $1
		ORDER BY o.created_at DESC
	`, domain.OutboxStatusPending)
	if err != nil {
		return nil, fmt.Errorf("get pending outbox payments: %w", err)
	}
	defer rows.Close()

	var out []*domain.OutboxPayment
	for rows.Next() {
		var o outboxPayment
		if err := rows.Scan(&o.ID, &o.PaymentID, &o.MerchantID, &o.Status, &o.Attempts, &o.LastError, &o.CreatedAt, &o.ProcessedAt); err != nil {
			return nil, fmt.Errorf("scan pending outbox payment %q: %w", o.PaymentID, err)
		}

		out = append(out, &domain.OutboxPayment{
			ID:          o.ID,
			PaymentID:   o.PaymentID,
			MerchantID:  o.MerchantID,
			Status:      o.Status,
			Attempts:    o.Attempts,
			LastError:   o.LastError,
			CreatedAt:   o.CreatedAt,
			ProcessedAt: o.ProcessedAt.Time,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending outbox payments: %w", err)
	}
	if out == nil {
		out = []*domain.OutboxPayment{}
	}

	return out, nil
}
