package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"paymentgateway/internal/domain"
)

type emailStep struct {
	Payments domain.PaymentRepository
}

func (e emailStep) Name() string { return "email" }

func (e emailStep) Do(ctx context.Context, w *PaymentWorkflow) error {
	p := w.Captured
	if p == nil {
		return errors.New("payment is not captured")
	}

	payload, err := json.Marshal(domain.MerchantPaymentSusccessfull{
		PaymentID: w.Pending.ID,
		Price:     w.Pending.Price,
		ProductID: "xxxx",
	})
	if err != nil {
		return fmt.Errorf("email: failed to marshal event: %w", err)
	}

	if err := e.Payments.OutboxEvent(ctx, "payment", w.Pending.ID, "payment.succeeded", payload); err != nil {
		// Outbox write failure must not fail the payment — the payment is
		// already captured and durable. Delivery is best-effort via the
		// outbox worker (deferred).
		return nil
	}

	return nil
}

func (e emailStep) Compensate(ctx context.Context, w *PaymentWorkflow) error {
	return nil
}
