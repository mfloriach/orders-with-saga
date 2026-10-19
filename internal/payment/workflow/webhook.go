package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/webhooks"
	"paymentgateway/pkg/apiclient"
)

// MerchantLookup resolves a merchant by ID. *merchant.Repository satisfies it.
type MerchantLookup interface {
	FindByID(ctx context.Context, id string) (*domain.Merchant, error)
}

type webhook struct {
	Merchants MerchantLookup
	Client    *apiclient.Client
}

func (w webhook) Name() string { return "webhook" }

func (w webhook) Do(ctx context.Context, wf *PaymentWorkflow) error {
	p := wf.StatusCaptured
	if p == nil {
		return errors.New("payment is not captured")
	}

	

	event := webhooks.Event{
		PaymentID:  p.ID,
		MerchantWebhook: wf.Merchant.WebhookURL,
		Amount:     p.Amount,
		Status:     string(domain.StatusCaptured),
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("webhook: failed to marshal event: %w", err)
	}

	resp, err := w.Client.Post(ctx, *m.WebhookURL, body,
		apiclient.WithHeader("Content-Type", "application/json"),
	)
	if err != nil {
		return fmt.Errorf("webhook: failed to notify merchant %s: %w", p.MerchantID, err)
	}

	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: merchant %s returned %s", p.MerchantID, resp.Status)
	}

	return nil
}

func (w webhook) Compensate(ctx context.Context, wf *PaymentWorkflow) error {
	if err := w.Do(ctx, wf); err != nil {
		return err
	}
	return nil
}
