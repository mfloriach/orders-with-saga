package repositories

import (
	"context"
	"database/sql"
	"paymentgateway/internal/domain"

	"github.com/google/uuid"
)

type AuditRepository struct{ db *sql.DB }

func NewAuditRepository(db *sql.DB) *AuditRepository { return &AuditRepository{db} }

// RecordStep appends one saga step execution to the audit trail. Rows
// accumulate (no per-step uniqueness) so replays stay visible.
func (r *AuditRepository) RecordStep(ctx context.Context, rec domain.StepRecord) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO payment_step_events (id, payment_id, saga_id, step, outcome, reason)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		uuid.NewString(), rec.PaymentID, rec.SagaID, rec.Step, rec.Outcome, rec.Reason)
	return err
}
