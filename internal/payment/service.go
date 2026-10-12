package payment

import (
	"context"
	"errors"

	"paymentgateway/internal/merchant"
	"paymentgateway/pkg/saga"

	"github.com/redis/go-redis/v9"
)

type ConflictError struct{ Existing *Payment }

func (e *ConflictError) Error() string { return "idempotency_conflict" }

type Service struct {
	app       *AppRepository
	ledger    *LedgerRepository
	merchants *merchant.Repository
	redis     *redis.Client
}

func NewService(app *AppRepository, ledger *LedgerRepository, merchants *merchant.Repository, rdb *redis.Client) *Service {
	return &Service{app: app, ledger: ledger, merchants: merchants, redis: rdb}
}

func idemKey(merchantID, key string) string { return "payments:idem:" + merchantID + ":" + key }

func (s *Service) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error) {
	if _, err := s.merchants.FindByID(ctx, merchantID); err != nil {
		return nil, err
	}

	existing, err := s.app.FindByIdem(ctx, merchantID, idemKey)
	if existing != nil {
		return nil, &ConflictError{Existing: existing}
	}
	if err != nil {
		return nil, err
	}

	p := &Payment{MerchantID: merchantID, Amount: amount, IdempotencyKey: idemKey}
	markFailed := func(c context.Context) error { s.app.MarkFailed(c, p.ID); return nil }
	err = saga.Saga{Name: "create-payment", Steps: []saga.Step{
		{Name: "insert-payment",
			Do: func(ctx context.Context) error {
				return s.app.InsertPending(ctx, p)
			},
			Compensate: markFailed},
		{Name: "insert-ledger", Do: func(ctx context.Context) error {
			return s.ledger.Insert(ctx, p.ID, p.Amount)
		}, Compensate: markFailed},
	}}.Run(ctx)
	if err != nil {
		var re *saga.RunError
		if errors.As(err, &re) && errors.Is(re.StepErr, ErrConflict) {
			return nil, &ConflictError{Existing: existing}
		}
		return nil, err
	}

	return p, nil
}

func idemKeyFunc(merchantID, key string) string { return idemKey(merchantID, key) }
