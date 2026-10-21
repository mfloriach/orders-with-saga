package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"paymentgateway/internal/domain"
)

// MerchantLookup resolves a merchant by ID. *merchant.Repository satisfies it.
type MerchantLookup interface {
	FindByID(ctx context.Context, id string) (*domain.Merchant, error)
}

type webhook struct {
	Payments domain.PaymentRepository
}

func (w webhook) Name() string { return "webhook" }

func (w webhook) Do(ctx context.Context, wf *PaymentWorkflow) error {
	p := wf.Captured
	if p == nil {
		return errors.New("payment is not captured")
	}

	// Merchant without webhook URL skips delivery — no outbox row is created.
	if wf.Merchant.WebhookURL == nil || *wf.Merchant.WebhookURL == "" {
		return nil
	}

	payload, err := json.Marshal(domain.MerchantPaymentSusccessfull{
		PaymentID: wf.Pending.ID,
		Price:     wf.Pending.Price,
		ProductID: "xxxx",
	})
	if err != nil {
		return fmt.Errorf("webhook: failed to marshal event: %w", err)
	}

	if err := w.Payments.OutboxEvent(ctx, "payment", wf.Pending.ID, "payment.succeeded", payload); err != nil {
		// Outbox write failure must not fail the payment — the payment is
		// already captured and durable. Delivery is best-effort via the
		// outbox worker (deferred).
		return nil
	}

	return nil
}

func (w webhook) Compensate(ctx context.Context, wf *PaymentWorkflow) error {
	return nil
}
