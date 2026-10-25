package workflow

import (
	"context"
	"errors"
	"fmt"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/notifications/email"
)

type emailStep struct {
	Sender email.Senders
}

func (e emailStep) Name() string { return "email" }

func (e emailStep) Do(ctx context.Context, wf *PaymentWorkflow) error {
	p, ok := wf.Payment.(domain.PaymentCaptured)
	if !ok {
		return errors.New("payment is not captured")
	}

	data := email.PaymentSuccessfully{Payment: p, Merchant: *wf.Merchant, User: wf.User}
	if err := e.Sender.SendPaymentSuccessfully(ctx, data); err != nil {
		return fmt.Errorf("email: failed to send payment success: %w", err)
	}

	return nil
}

func (e emailStep) Compensate(ctx context.Context, w *PaymentWorkflow) error {
	return e.Do(ctx, w)
}
