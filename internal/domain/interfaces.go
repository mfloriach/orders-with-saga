package domain

import (
	"context"
)

type PaymentRepository interface {
	IsIdempotent(ctx context.Context, merchantID, key string) (string, error)
	New(ctx context.Context, p *PaymentPending) error
	Authorized(ctx context.Context, p PaymentPending, authorizationID string) (PaymentAuthenticated, error)
	Captured(ctx context.Context, p PaymentAuthenticated) (PaymentCaptured, error)
	Failed(ctx context.Context, paymentId string, err error) (PaymentFailed, error)
}

type ConflictError struct {
	PaymentId     string
	IdempotentKey string
}

func (e *ConflictError) Error() string { return "idempotency_conflict" }
