package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/payment/steps"
	"paymentgateway/internal/webhooks"
	"paymentgateway/pkg/saga"

	"github.com/redis/go-redis/v9"
)

// merchantStore is the merchant lookup seam. *merchant.Repository satisfies
// it; the interface keeps the service testable without a database.
type merchantStore interface {
	FindByID(ctx context.Context, id string) (*domain.Merchant, error)
}

type Service struct {
	payments    domain.PaymentRepository
	audit       domain.AuditStore
	merchants   merchantStore
	redis       *redis.Client
	cardNetwork domain.CardNetwork
	email       steps.Email
	aa          map[string]steps.Step
}

func NewService(
	payments domain.PaymentRepository,
	audit domain.AuditStore,
	merchants merchantStore,
	rdb *redis.Client,
	cardNetwork domain.CardNetwork,
	events webhooks.Publisher,
	emails webhooks.EmailPublisher,
) *Service {
	aa := map[string]steps.Step{
		"pendingPayment": steps.PendingPayment{Payments: payments},
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
		email:       steps.Email{Publisher: emails},
		aa:          aa,
	}
}

func (s *Service) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*domain.Payment, error) {
	existing, err := s.payments.FindByIdem(ctx, merchantID, idemKey)
	if existing != nil {
		return nil, &domain.ConflictError{Existing: existing}
	}
	if err != nil {
		return nil, fmt.Errorf("payment: failed to check idempotency: %w", err)
	}

	if _, err := s.merchants.FindByID(ctx, merchantID); err != nil {
		return nil, fmt.Errorf("payment: failed to check merchant: %w", err)
	}

	p := &domain.Payment{MerchantID: merchantID, Amount: amount, IdempotencyKey: idemKey}
	err = saga.Saga{
		Name: "create-payment",
		Steps: []saga.Step{
			s.aa["pendingPayment"].Run(p),
			s.aa["authorize"].Run(p),
			s.aa["capture"].Run(p),
			s.aa["webhook"].Run(p),
		},
		// Audit is observability, not money: a failing write logs loudly
		// and never fails the payment.
		OnStep: func(ctx context.Context, sagaID, step string, err error) {
			rec := domain.StepRecord{PaymentID: p.ID, SagaID: sagaID, Step: step, Outcome: "success"}
			if err != nil {
				rec.Outcome = "failure"
				rec.Reason = err.Error()
			}

			if rerr := s.audit.RecordStep(ctx, rec); rerr != nil {
				slog.ErrorContext(ctx, "audit record failed",
					slog.String("payment_id", p.ID), slog.String("step", step), slog.String("error", rerr.Error()))
			}
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
			if errors.Is(re.StepErr, domain.ErrConflict) {
				return nil, &domain.ConflictError{Existing: existing}
			}
		}
		return nil, fmt.Errorf("payment: failed to create %s: %w", p.ID, err)
	}

	// Email goes out after success, outside the saga: a notification failure
	// must never unwind a captured payment.
	if err := s.email.Publish(ctx, p); err != nil {
		slog.ErrorContext(ctx, "email publish failed",
			slog.String("payment_id", p.ID), slog.String("error", err.Error()))
		return nil, fmt.Errorf("payment: failed to publish email event %s: %w", p.ID, err)
	}

	return p, nil
}
