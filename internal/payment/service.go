package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"paymentgateway/internal/merchant"
	"paymentgateway/pkg/saga"

	"github.com/redis/go-redis/v9"
)

type ConflictError struct{ Existing *Payment }

func (e *ConflictError) Error() string { return "idempotency_conflict" }

type Service struct {
	payments    PaymentRepository
	audit       AuditStore
	merchants   *merchant.Repository
	redis       *redis.Client
	cardNetwork CardNetwork
}

func NewService(payments PaymentRepository, audit AuditStore, merchants *merchant.Repository, rdb *redis.Client, cardNetwork CardNetwork) *Service {
	return &Service{payments: payments, audit: audit, merchants: merchants, redis: rdb, cardNetwork: cardNetwork}
}

func (s *Service) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error) {
	existing, err := s.payments.FindByIdem(ctx, merchantID, idemKey)
	if existing != nil {
		return nil, &ConflictError{Existing: existing}
	}
	if err != nil {
		return nil, err
	}

	if _, err := s.merchants.FindByID(ctx, merchantID); err != nil {
		return nil, err
	}

	p := &Payment{MerchantID: merchantID, Amount: amount, IdempotencyKey: idemKey}
	err = saga.Saga{Name: "create-payment", Steps: []saga.Step{
		{
			Name: "insert-payment",
			Do: func(ctx context.Context) error {
				return s.payments.InsertPending(ctx, p)
			},
			Compensate: func(c context.Context) error {
				if err := s.payments.MarkFailed(c, p); err != nil {
					return fmt.Errorf("payment: failed to mark failed: %w", err)
				}

				return nil
			},
		},
		{
			Name: "insert-ledger",
			Do: func(ctx context.Context) error {
				return s.audit.Insert(ctx, p.ID, p.Amount)
			},
			Compensate: func(c context.Context) error {
				return s.audit.MarkFailed(c, p.ID)
			},
		},
		{
			// Non-compensatable: Compensate is nil, and an unconfirmed
			// outcome returns InDoubtError so Run skips unwind entirely.
			Name: "call-api",
			Do: func(ctx context.Context) error {
				return s.callNetwork(ctx, p, idemKey)
			},
		},
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
			if errors.Is(re.StepErr, ErrConflict) {
				return nil, &ConflictError{Existing: existing}
			}
		}
		return nil, err
	}

	return p, nil
}

// callNetwork runs authenticate (retries and per-call timeouts live in
// the repository), reconciles an ambiguous auth outcome via GetPayment,
// then captures. Terminal failures and confirmed absences are ordinary
// errors (caller compensates); an unconfirmed outcome is InDoubtError
// (caller keeps state for the batch).
func (s *Service) callNetwork(ctx context.Context, p *Payment, idemKey string) error {
	if s.cardNetwork == nil {
		return errors.New("payment: card network not configured")
	}

	// authorize
	err := s.cardNetwork.Authenticate(ctx, p.ID, p.Amount, idemKey)
	if err == nil {
		return s.authorize(ctx, p)
	}
	if !s.cardNetwork.Retryable(err) {
		return err
	}

	// reconcile the ambiguous auth via GetPayment.
	_, status, qerr := s.cardNetwork.GetPayment(ctx, p.ID)
	if qerr != nil {
		return &saga.InDoubtError{Step: "call-api", Err: err}
	}
	switch status {
	case networkAccepted:
		if err := s.authorize(ctx, p); err != nil {
			return err
		}
		return s.capture(ctx, p, idemKey)
	case networkRejected:
		return fmt.Errorf("payment: card network did not accept %s: %w", p.ID, err)
	default:
		return &saga.InDoubtError{Step: "call-api", Err: err}
	}
}

// authorize moves a locally pending payment to authorized after the
// network confirms funds held.
func (s *Service) authorize(ctx context.Context, p *Payment) error {
	if err := s.payments.MarkAuthorized(ctx, p); err != nil {
		return err
	}

	return nil
}

// capture settles an authorized payment through the network, then marks
// it captured. A retryable capture failure is in-doubt: the payment
// status query cannot confirm capture state, so the payment stays
// authorized with ledger intact for the batch.
func (s *Service) capture(ctx context.Context, p *Payment, idemKey string) error {
	if err := s.cardNetwork.Capture(ctx, p.ID, p.Amount, idemKey); err != nil {
		if !s.cardNetwork.Retryable(err) {
			return fmt.Errorf("payment: capture %s refused: %w", p.ID, err)
		}
		return &saga.InDoubtError{Step: "call-api", Err: err}
	}

	if err := s.payments.MarkCaptured(ctx, p); err != nil {
		return err
	}

	return nil
}
