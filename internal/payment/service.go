package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment/models"
	"paymentgateway/internal/payment/steps"
	"paymentgateway/internal/webhooks"
	"paymentgateway/pkg/saga"

	"github.com/redis/go-redis/v9"
)

// merchantStore is the merchant lookup seam. *merchant.Repository satisfies
// it; the interface keeps the service testable without a database.
type merchantStore interface {
	FindByID(ctx context.Context, id string) (*merchant.Merchant, error)
}

type Service struct {
	payments    models.PaymentRepository
	audit       models.AuditStore
	merchants   merchantStore
	redis       *redis.Client
	cardNetwork models.CardNetwork
	aa          map[string]steps.Step
}

func NewService(payments models.PaymentRepository, audit models.AuditStore, merchants merchantStore, rdb *redis.Client, cardNetwork models.CardNetwork, events webhooks.Publisher) *Service {
	aa := map[string]steps.Step{
		"pendingPayment": steps.PendingPayment{Payments: payments},
		"insertAudit":    steps.StepInsertAudit{Audit: audit},
		"authorize":      steps.Authorize{Payments: payments, CardNetwork: cardNetwork},
		"capture":        steps.Capture{Payments: payments, CardNetwork: cardNetwork},
		"webhook":        steps.Webhook{Publisher: events},
	}

	return &Service{
		payments:    payments,
		audit:       audit,
		merchants:   merchants,
		redis:       rdb,
		cardNetwork: cardNetwork,
		aa:          aa,
	}
}

func (s *Service) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*models.Payment, error) {
	existing, err := s.payments.FindByIdem(ctx, merchantID, idemKey)
	if existing != nil {
		return nil, &models.ConflictError{Existing: existing}
	}
	if err != nil {
		return nil, fmt.Errorf("payment: failed to check idempotency: %w", err)
	}

	if _, err := s.merchants.FindByID(ctx, merchantID); err != nil {
		return nil, fmt.Errorf("payment: failed to check merchant: %w", err)
	}

	p := &models.Payment{MerchantID: merchantID, Amount: amount, IdempotencyKey: idemKey}
	err = saga.Saga{
		Name: "create-payment",
		Steps: []saga.Step{
			s.aa["pendingPayment"].Run(p),
			s.aa["insertAudit"].Run(p),
			s.aa["authorize"].Run(p),
			s.aa["capture"].Run(p),
			s.aa["webhook"].Run(p),
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
				return nil, &models.ConflictError{Existing: existing}
			}
		}
		return nil, fmt.Errorf("payment: failed to create %s: %w", p.ID, err)
	}

	return p, nil
}
