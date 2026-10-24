package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"paymentgateway/internal/domain"

	"github.com/google/uuid"
)

// storeOutbox inserts the pending outbox record for a payment that was just
// captured. It runs inside the caller's transaction so the capture status flip
// and the enqueue commit or roll back together.
func (r *paymentDatabaseRepository) storeOutbox(ctx context.Context, tx *sql.Tx, p domain.PaymentAuthenticated) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_payments (id, payment_id, merchant_id)
		VALUES ($1, $2, $3)
	`, uuid.NewString(), p.ID, p.MerchantID)
	if err != nil {
		return fmt.Errorf("enqueue captured payment %q for processing: %w", p.ID, err)
	}

	return nil
}

// Pending returns captured payments whose outbox processing has not completed
// yet — captured but not processed. Callers claim and flip statuses as their
// processing attempts finish.
func (r *paymentDatabaseRepository) Pending(ctx context.Context) ([]*domain.OutboxPayment, error) {
	rows, err := r.db.QueryContext(ctx, `
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

// outboxPayment mirrors the outbox_payments row so scans target concrete
// columns before translating into the domain type.
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
