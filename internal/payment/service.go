package payment

import (
	"context"
	"fmt"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/payment/workflow"

	saga "paymentgateway/pkg/saga/v2"

	"github.com/google/uuid"
)

type merchantStore interface {
	FindByID(context.Context, string) (*domain.Merchant, error)
}

type Service struct {
	payments    domain.PaymentRepository
	merchants   merchantStore
	cardNetwork domain.CardNetwork
	saga        *saga.Saga[workflow.PaymentWorkflow]
}

func NewService(
	payments domain.PaymentRepository,
	merchants merchantStore,
	saga *saga.Saga[workflow.PaymentWorkflow],
) *Service {
	return &Service{
		payments:  payments,
		merchants: merchants,
		saga:      saga,
	}
}

func (s *Service) Create(ctx context.Context, merchantID string, amount int64, idemKey string) (*domain.PaymentCaptured, error) {
	if _, err := s.merchants.FindByID(ctx, merchantID); err != nil {
		return nil, fmt.Errorf("payment: failed to check merchant: %w", err)
	}

	existing, err := s.payments.FindByIdem(ctx, merchantID, idemKey)
	if existing != nil {
		return nil, &domain.ConflictError{Existing: existing}
	}
	if err != nil {
		return nil, fmt.Errorf("payment: failed to check idempotency: %w", err)
	}

	pPending := &domain.PaymentPending{ID: uuid.New().String(), MerchantID: merchantID, Amount: amount, IdempotencyKey: idemKey}
	wf := workflow.PaymentWorkflow{Pending: pPending}

	if err := s.payments.New(ctx, pPending); err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, "paymentID", pPending.ID)
	if err := s.saga.Execute(ctx, &wf); err != nil {
		if _, err := s.payments.Failed(ctx, pPending.ID, err); err != nil {
			return nil, err
		}

		return nil, err
	}

	return wf.Captured, nil
}
