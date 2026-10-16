package payment

import (
	"context"
	"errors"
	"fmt"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/payment/steps"
	"paymentgateway/internal/webhooks"
	"paymentgateway/pkg/saga"
)

type merchantStore interface {
	FindByID(context.Context, string) (*domain.Merchant, error)
}

type sagaProcedure interface {
	Run(context.Context, *domain.Payment) error
}

type Service struct {
	payments    domain.PaymentRepository
	merchants   merchantStore
	cardNetwork domain.CardNetwork
	steps       []saga.Step[domain.Payment]
	saga        sagaProcedure
}

func NewService(
	payments domain.PaymentRepository,
	merchants merchantStore,
	cardNetwork domain.CardNetwork,
	events webhooks.Publisher,
) *Service {
	steps := steps.Registry(payments, cardNetwork, events)

	return &Service{
		payments:    payments,
		merchants:   merchants,
		cardNetwork: cardNetwork,
		steps:       steps,
		saga:        saga.New("create-payment", steps),
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
	if err = s.saga.Run(ctx, p); err != nil {
		var re *saga.RunError
		if !errors.As(err, &re) {
			return nil, fmt.Errorf("payment: failed to create %s: %w", p.ID, err)
		}

		return nil, fmt.Errorf("payment: failed to create %s: %w", p.ID, err)
	}

	return p, nil
}
