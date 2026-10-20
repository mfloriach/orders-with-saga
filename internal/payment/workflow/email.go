package workflow

import (
	"context"
	"errors"
	"fmt"

	"paymentgateway/internal/email"
)

type emailStep struct {
	Sender email.Senders
}

func (e emailStep) Name() string { return "email" }

func (e emailStep) Do(ctx context.Context, w *PaymentWorkflow) error {
	p := w.Captured
	if p == nil {
		return errors.New("payment is not captured")
	}

	data := email.PaymentSuccessfully{Payment: *p, Merchant: *w.Merchant, User: w.User}
	if err := e.Sender.SendPaymentSuccessfully(ctx, data); err != nil {
		return fmt.Errorf("email could not be send")
	}

	return nil
}

func (e emailStep) Compensate(ctx context.Context, w *PaymentWorkflow) error {
	return e.Do(ctx, w)
}
