package steps

import (
	"paymentgateway/internal/payment/models"
	"paymentgateway/pkg/saga"
)

type Step interface {
	Run(p *models.Payment) saga.Step
}
