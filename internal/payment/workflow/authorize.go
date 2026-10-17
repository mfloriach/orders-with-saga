package workflow

import (
	"context"
	"errors"
	"fmt"
	"paymentgateway/internal/domain"
)

type authorize struct {
	Payments    domain.PaymentRepository
	CardNetwork domain.CardNetwork
}

func (a authorize) Name() string { return "authorize" }

func (a authorize) Do(ctx context.Context, w *PaymentWorkflow) error {
	p := w.Pending
	if p == nil {
		return errors.New("payment is not pending")
	}

	authorizeId, err := a.CardNetwork.Authorize(ctx, p.ID, p.Amount, p.IdempotencyKey)
	if err != nil {
		a.Payments.Failed(ctx, p.ID, fmt.Errorf("payment: failed to authorize: %w", err))
		return err
	}

	pAuthorize, err := a.Payments.Authorized(ctx, *p, authorizeId)
	if err != nil {
		return err
	}

	w.Authenticated = &pAuthorize

	return nil
}

func (a authorize) Compensate(ctx context.Context, w *PaymentWorkflow) error {
	if err := a.Do(ctx, w); err != nil {
		return err
	}

	return nil
}
