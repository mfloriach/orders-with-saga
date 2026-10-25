package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"uuid"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/payment/repositories"
	"paymentgateway/pkg/apiclient"
)

// MerchantLookup resolves a merchant by ID. *merchant.Repository satisfies it.
type MerchantLookup interface {
	FindByID(ctx context.Context, id string) (*domain.Merchant, error)
}

type webhook struct {
	Client  *apiclient.Client
	webhook repositories.MerchantWebhook
}

func (w webhook) Name() string { return "webhook" }

func (w webhook) Do(ctx context.Context, wf *PaymentWorkflow) error {
	p, ok := wf.Payment.(domain.PaymentCaptured)
	if !ok {
		return errors.New("payment is not captured")
	}

	payload, err := json.Marshal(domain.MerchantPaymentSusccessfull{
		PaymentID: p.ID,
		Price:     p.Price,
		ProductID: "xxxx",
	})
	if err != nil {
		return fmt.Errorf("webhook: failed to marshal event: %w", err)
	}

	event := domain.MerchantEvent{
		ID:        uuid.New().String(),
		Type:      domain.MerchantEventTypeSuccessfullPayment,
		Merchant:  *wf.Merchant,
		Payload:   payload,
		CreatedAt: time.Now(),
	}

	if err := w.webhook.Notify(ctx, event); err != nil {
		return fmt.Errorf("webhook: failed to notify merchant with event %s: %w", event.ID, err)
	}

	return nil
}

func (w webhook) Compensate(ctx context.Context, wf *PaymentWorkflow) error {
	if err := w.Do(ctx, wf); err != nil {
		return err
	}
	return nil
}
