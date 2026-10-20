package workflow

import (
	"context"
	"errors"
	"fmt"

	"paymentgateway/internal/email"
)

type emailStep struct {
	Sender email.EmailSender
}

func (e emailStep) Name() string { return "email" }

func (e emailStep) Do(ctx context.Context, w *PaymentWorkflow) error {
	p := w.Captured
	if p == nil {
		return errors.New("payment is not captured")
	}

	subject := fmt.Sprintf("Payment Receipt - %s", p.ID)
	body := fmt.Sprintf("Your payment of %d has been processed successfully.\nPayment ID: %s\nMerchant: %s", p.Amount, p.ID, w.Merchant.Name)
	data := email.EmailData{
		To:      w.User.Email,
		Subject: subject,
		Body:    body,
	}

	if err := e.Sender(ctx, data); err != nil {
		return fmt.Errorf("email: failed to send: %w", err)
	}

	return nil
}

func (e emailStep) Compensate(ctx context.Context, w *PaymentWorkflow) error {
	return e.Do(ctx, w)
}
