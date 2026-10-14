package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment/models"
	"paymentgateway/internal/payment/steps"
	"paymentgateway/pkg/saga"

	"github.com/redis/go-redis/v9"
)

type ConflictError struct{ Existing *models.Payment }

func (e *ConflictError) Error() string { return "idempotency_conflict" }

type Service struct {
	payments    models.PaymentRepository
	audit       models.AuditStore
	merchants   *merchant.Repository
	redis       *redis.Client
	cardNetwork models.CardNetwork
}

func NewService(payments models.PaymentRepository, audit models.AuditStore, merchants *merchant.Repository, rdb *redis.Client, cardNetwork models.CardNetwork) *Service {
	return &Service{payments: payments, audit: audit, merchants: merchants, redis: rdb, cardNetwork: cardNetwork}
}

func (s *Service) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*models.Payment, error) {
	existing, err := s.payments.FindByIdem(ctx, merchantID, idemKey)
	if existing != nil {
		return nil, &ConflictError{Existing: existing}
	}
	if err != nil {
		return nil, fmt.Errorf("payment: failed to check idempotency: %w", err)
	}

	if _, err := s.merchants.FindByID(ctx, merchantID); err != nil {
		return nil, fmt.Errorf("payment: failed to check merchant: %w", err)
	}

	p := &models.Payment{MerchantID: merchantID, Amount: amount, IdempotencyKey: idemKey}
	err = saga.Saga{Name: "create-payment", Steps: []saga.Step{
		steps.PendingPayment{Payments: s.payments}.Run(p),
		steps.StepInsertAudit{Audit: s.audit}.Run(p),
		steps.Authorize{Payments: s.payments, CardNetwork: s.cardNetwork}.Run(p),
		steps.Capture{Payments: s.payments, CardNetwork: s.cardNetwork}.Run(p),
		steps.Webhook{}.Run(p),
	}}.Run(ctx)
	if err != nil {
		var re *saga.RunError
		if errors.As(err, &re) {
			if re.InDoubt {
				// Keep payment pending and ledger intact; queue for the
				// batch. A failed enqueue is logged loudly — a future
				// sweep over stale pendings is the fallback.
				if qerr := s.payments.EnqueueReconciliation(ctx, p.ID, idemKey, re.FailedStep, re.StepErr); qerr != nil {
					slog.ErrorContext(ctx, "reconciliation enqueue failed",
						slog.String("payment_id", p.ID), slog.String("error", qerr.Error()))
				}
				return nil, re.StepErr
			}
			if errors.Is(re.StepErr, models.ErrConflict) {
				return nil, &ConflictError{Existing: existing}
			}
		}
		return nil, fmt.Errorf("payment: failed to create %s: %w", p.ID, err)
	}

	return p, nil
}

// type Step interface {
// 	Do(ctx context.Context) error
// 	Compensate(ctx context.Context) error
// }
