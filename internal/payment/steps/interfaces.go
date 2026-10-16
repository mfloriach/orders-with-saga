package steps

import (
	"paymentgateway/internal/domain"
	"paymentgateway/pkg/saga"
)

type Step interface {
	Run(p *domain.Payment) saga.Step
}
