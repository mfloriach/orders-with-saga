package repositories

import (
	"context"
	"fmt"
	"paymentgateway/internal/domain"
	"paymentgateway/pkg/apiclient"
)

type MerchantWebhook struct {
	client *apiclient.Client
}

func (mw MerchantWebhook) Notify(ctx context.Context, e domain.MerchantEvent) error {
	if e.Merchant.WebhookURL == nil || *e.Merchant.WebhookURL == "" {
		return nil
	}

	resp, err := mw.client.Post(ctx, *e.Merchant.WebhookURL, e.Payload,
		apiclient.WithHeader("Content-Type", "application/json"),
	)
	if err != nil {
		return fmt.Errorf("webhook: failed to notify merchant %s: %w", e.ID, err)
	}

	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: merchant %s returned %s", e.ID, resp.Status)
	}

	return nil
}
