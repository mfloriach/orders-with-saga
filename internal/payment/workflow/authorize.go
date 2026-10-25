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

func (a authorize) Do(ctx context.Context, wf *PaymentWorkflow) error {
	p, ok := wf.Payment.(domain.PaymentPending)
	if !ok {
		return errors.New("payment is not captured")
	}

	authorizeId, err := a.CardNetwork.Authorize(ctx, p.ID, p.Price.Amount, p.IdempotencyKey)
	if err != nil {
		a.Payments.Failed(ctx, p.ID, fmt.Errorf("payment: failed to authorize: %w", err))
		return err
	}

	pAuthorize, err := a.Payments.Authorized(ctx, p, authorizeId)
	if err != nil {
		return err
	}

	wf.Payment = &pAuthorize

	return nil
}

func (a authorize) Compensate(ctx context.Context, w *PaymentWorkflow) error {
	if err := a.Do(ctx, w); err != nil {
		return err
	}

	return nil
}
