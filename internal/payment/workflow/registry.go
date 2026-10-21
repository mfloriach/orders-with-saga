package workflow

import (
	"paymentgateway/internal/domain"
	saga "paymentgateway/pkg/saga/v2"
)

type PaymentWorkflow struct {
	User     domain.User
	Merchant *domain.Merchant

	Pending       *domain.PaymentPending
	Authenticated *domain.PaymentAuthenticated
	Captured      *domain.PaymentCaptured
}

func NewWorkflow(
	payments domain.PaymentRepository,
	cardNetwork domain.CardNetwork,
	merchants MerchantLookup,
) *saga.Saga[PaymentWorkflow] {
	authorize := authorize{Payments: payments, CardNetwork: cardNetwork}
	capture := capture{Payments: payments, CardNetwork: cardNetwork}
	webhookStep := webhook{Payments: payments}
	emailStep := emailStep{Payments: payments}

	return saga.New(authorize, capture, webhookStep, emailStep)
}
