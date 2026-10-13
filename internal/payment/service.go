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
	app       PaymentRepository
	ledger    *LedgerRepository
	merchants *merchant.Repository
	redis     *redis.Client
	processor Processor
}

func NewService(app PaymentRepository, ledger *LedgerRepository, merchants *merchant.Repository, rdb *redis.Client, processor Processor) *Service {
	return &Service{app: app, ledger: ledger, merchants: merchants, redis: rdb, processor: processor}
}

func (s *Service) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error) {
	existing, err := s.app.FindByIdem(ctx, merchantID, idemKey)
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
				return s.app.InsertPending(ctx, p)
			},
			Compensate: func(c context.Context) error {
				if err := p.TransitionTo(StatusFailed); err != nil {
					return fmt.Errorf("payment: failed to transition to failed: %w", err)
				}

				if err := s.app.MarkFailed(ctx, p.ID); err != nil {
					return fmt.Errorf("payment: failed to mark failed: %w", err)
				}

				return nil
			},
		},
		{
			Name: "insert-ledger",
			Do: func(ctx context.Context) error {
				return s.ledger.Insert(ctx, p.ID, p.Amount)
			},
			Compensate: func(c context.Context) error {
				return s.ledger.MarkFailed(c, p.ID)
			},
		},
		{
			// Non-compensatable: Compensate is nil, and an unconfirmed
			// outcome returns InDoubtError so Run skips unwind entirely.
			Name: "call-api",
			Do: func(ctx context.Context) error {
				return s.callProcessor(ctx, p, idemKey)
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
				if qerr := s.app.EnqueueReconciliation(ctx, p.ID, idemKey, re.FailedStep, re.StepErr); qerr != nil {
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

// callProcessor runs the external call (retries and per-call timeouts live
// in the repository), then reconciles an ambiguous outcome via status
// query. Terminal failures and confirmed absences are ordinary errors
// (caller compensates); an unconfirmed outcome is InDoubtError (caller
// keeps state for the batch).
func (s *Service) callProcessor(ctx context.Context, p *Payment, idemKey string) error {
	if s.processor == nil {
		return errors.New("payment: processor not configured")
	}

	// authorize
	err := s.processor.Call(ctx, p.ID, p.Amount, idemKey)
	if err == nil {
		if err := p.TransitionTo(StatusAuthorized); err != nil {
			return err
		}

		if err := s.app.MarkAuthorized(ctx, p.ID); err != nil {
			return err
		}
		p.Status = StatusAuthorized

		return nil
	}
	if !s.processor.Retryable(err) {
		return err
	}

	// capture: the network holds funds, settle via the confirmed state.
	accepted, qerr := s.processor.Status(ctx, p.ID)
	if qerr != nil {
		return &saga.InDoubtError{Step: "call-api", Err: err}
	}
	if accepted {
		if err := p.TransitionTo(StatusAuthorized); err != nil {
			return err
		}

		if err := s.app.MarkAuthorized(ctx, p.ID); err != nil {
			return err
		}
		p.Status = StatusAuthorized

		if err := p.TransitionTo(StatusCaptured); err != nil {
			return err
		}

		if err := s.app.MarkCaptured(ctx, p.ID); err != nil {
			return err
		}
		p.Status = StatusCaptured

		return nil
	}
	return fmt.Errorf("payment: processor did not accept %s: %w", p.ID, err)
}
