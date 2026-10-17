package workflow

import (
	"paymentgateway/internal/domain"
	saga "paymentgateway/pkg/saga/v2"
)

type PaymentWorkflow struct {
	Pending       *domain.PaymentPending
	Authenticated *domain.PaymentAuthenticated
	Captured      *domain.PaymentCaptured
}

func NewWorkflow(payments domain.PaymentRepository, cardNetwork domain.CardNetwork) *saga.Saga[PaymentWorkflow] {
	authorize := authorize{Payments: payments, CardNetwork: cardNetwork}
	capture := capture{Payments: payments, CardNetwork: cardNetwork}

	return saga.New(authorize, capture)
}
