package steps

import (
	"context"
	"paymentgateway/internal/payment/models"
	"paymentgateway/pkg/saga"
)

type Webhook struct{}

func (s Webhook) Run(p *models.Payment) saga.Step {
	return saga.Step{
		Name: "send-webhook",
		Do: func(ctx context.Context) error {
			return nil
		},
		Compensate: func(c context.Context) error {
			return nil
		},
	}
}
