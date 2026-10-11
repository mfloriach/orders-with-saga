package payment

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"paymentgateway/internal/merchant"
)

type ConflictError struct{ Existing *Payment }

func (e *ConflictError) Error() string { return "idempotency_conflict" }

type Service struct {
	app      *AppRepository
	ledger   *LedgerRepository
	merchants *merchant.Repository
	redis    *redis.Client
}

func NewService(app *AppRepository, ledger *LedgerRepository, merchants *merchant.Repository, rdb *redis.Client) *Service {
	return &Service{app: app, ledger: ledger, merchants: merchants, redis: rdb}
}

func idemKey(merchantID, key string) string { return "payments:idem:" + merchantID + ":" + key }

func (s *Service) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*Payment, error) {
	if _, err := s.merchants.FindByID(ctx, merchantID); err != nil {
		return nil, err // merchant.ErrNotFound propagates
	}
	// fast-path: redis hit means duplicate
	if id, err := s.redis.Get(ctx, idemKeyFunc(merchantID, idemKey)).Result(); err == nil {
		if existing, _ := s.app.FindByIdem(ctx, merchantID, idemKey); existing != nil {
			_ = id
			return nil, &ConflictError{Existing: existing}
		}
	}
	p := &Payment{MerchantID: merchantID, Amount: amount, IdempotencyKey: idemKey}
	if err := s.app.InsertPending(ctx, p); err != nil {
		if errors.Is(err, ErrConflict) {
			existing, _ := s.app.FindByIdem(ctx, merchantID, idemKey)
			return nil, &ConflictError{Existing: existing}
		}
		return nil, err
	}
	if err := s.ledger.Insert(ctx, p.ID, p.Amount); err != nil {
		s.app.MarkFailed(ctx, p.ID)
		return nil, err
	}
	s.redis.Set(ctx, idemKeyFunc(merchantID, idemKey), p.ID, 24*time.Hour)
	return p, nil
}

func idemKeyFunc(merchantID, key string) string { return idemKey(merchantID, key) }
