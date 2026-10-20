package email

import (
	"context"
	"fmt"
	"paymentgateway/internal/domain"
)

type Senders interface {
	SendPaymentSuccessfully(context.Context, PaymentSuccessfully) error
}

type PaymentSuccessfully struct {
	Payment  domain.PaymentCaptured
	Merchant domain.Merchant
	User     domain.User
}

func (e *sMTPSender) SendPaymentSuccessfully(ctx context.Context, cnt PaymentSuccessfully) error {
	subject := fmt.Sprintf("Payment Receipt - %s", cnt.Payment.ID)

	body, err := render("payment_success.html", map[string]any{
		"PaymentID":    cnt.Payment.ID,
		"Amount":       cnt.Payment.Amount,
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
