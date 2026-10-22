package email

import (
	"context"
	"fmt"
	"paymentgateway/internal/domain"
)

type Senders interface {
	SendPaymentSuccessfully(context.Context, PaymentSuccessfully) error
	SendRefundInProgress(context.Context, RefundInProgress) error
}

type PaymentSuccessfully struct {
	Payment  domain.PaymentCaptured
	Merchant domain.Merchant
	User     domain.User
}

type RefundInProgress struct {
	RefundID     string
	Price        domain.Price
	MerchantName string
	RefundReason string
	User         domain.User
}

func (e *sMTPSender) SendPaymentSuccessfully(ctx context.Context, cnt PaymentSuccessfully) error {
	subject := fmt.Sprintf("Payment Receipt - %s", cnt.Payment.ID)

	body, err := render("payment_success.html", map[string]any{
		"PaymentID":    cnt.Payment.ID,
		"Price":        cnt.Payment.Price,
		"MerchantName": cnt.Merchant.Name,
	})
	if err != nil {
		return fmt.Errorf("email: render template: %w", err)
	}

	data := emailData{
		To:          cnt.User.Email,
		Subject:     subject,
		Body:        body,
		ContentType: "text/html",
	}

	if err := e.send(ctx, data); err != nil {
		return fmt.Errorf("email: failed to send: %w", err)
	}

	return nil
}

func (e *sMTPSender) SendRefundInProgress(ctx context.Context, cnt RefundInProgress) error {
	subject := fmt.Sprintf("Refund In Progress - %s", cnt.RefundID)

	body, err := render("refund_in_progress.html", map[string]any{
		"RefundID":     cnt.RefundID,
		"Price":        cnt.Price,
		"MerchantName": cnt.MerchantName,
		"RefundReason": cnt.RefundReason,
	})
	if err != nil {
		return fmt.Errorf("email: render template: %w", err)
	}

	data := emailData{
		To:          cnt.User.Email,
		Subject:     subject,
		Body:        body,
		ContentType: "text/html",
	}

	if err := e.send(ctx, data); err != nil {
		return fmt.Errorf("email: failed to send: %w", err)
	}

	return nil
}
