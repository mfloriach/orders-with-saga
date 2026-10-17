package payment

import (
	"context"
	"fmt"
	"uuid"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/webhooks"
)

type merchantStore interface {
	FindByID(context.Context, string) (*domain.Merchant, error)
}

type Service struct {
	payments    domain.PaymentRepository
	merchants   merchantStore
	cardNetwork domain.CardNetwork
}

func NewService(
	payments domain.PaymentRepository,
	merchants merchantStore,
	cardNetwork domain.CardNetwork,
	events webhooks.Publisher,
) *Service {
	return &Service{
		payments:    payments,
		merchants:   merchants,
		cardNetwork: cardNetwork,
	}
}

func (s *Service) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*domain.PaymentCaptured, error) {
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

	pPending := &domain.PaymentPending{ID: uuid.New().String(), MerchantID: merchantID, Amount: amount, IdempotencyKey: idemKey}
	ctx = context.WithValue(ctx, "paymentID", pPending.ID)

	if err := s.payments.New(ctx, pPending); err != nil {
		return nil, err
	}

	authorizeId, err := s.cardNetwork.Authorize(ctx, pPending.ID, pPending.Amount, idemKey)
	if err != nil {
		s.payments.Failed(ctx, pPending.ID, fmt.Errorf("payment: failed to authorize: %w", err))
		return nil, err
	}

	pAuthorize, err := s.payments.Authorized(ctx, *pPending, authorizeId)
	if err != nil {
		return nil, err
	}

	if err := s.cardNetwork.Capture(ctx, pAuthorize.ID); err != nil {
		s.payments.Failed(ctx, pAuthorize.ID, fmt.Errorf("payment: failed to capture: %w", err))
		return nil, err
	}

	pCaptured, err := s.payments.Captured(ctx, pAuthorize)
	if err != nil {
		return nil, err
	}

	return &pCaptured, nil
}
