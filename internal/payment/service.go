package payment

import (
	"context"
	"errors"

	"paymentgateway/internal/merchant"

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
	if err := s.app.InsertPending(ctx, p); err != nil {
		if errors.Is(err, ErrConflict) {
			return nil, &ConflictError{Existing: existing}
		}
		return nil, err
	}

	if err := s.ledger.Insert(ctx, p.ID, p.Amount); err != nil {
		s.app.MarkFailed(ctx, p.ID)
		return nil, err
	}

	return p, nil
}

func idemKeyFunc(merchantID, key string) string { return idemKey(merchantID, key) }
