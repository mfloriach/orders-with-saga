package workflow

import (
	"paymentgateway/internal/domain"
	"paymentgateway/pkg/apiclient"
	saga "paymentgateway/pkg/saga/v2"
)

type PaymentWorkflow struct {
	Merchant: 		*domain.Merchant

	Pending       *domain.PaymentPending
	Authenticated *domain.PaymentAuthenticated
	Captured      *domain.PaymentCaptured
}

func NewWorkflow(
	payments domain.PaymentRepository, 
	cardNetwork domain.CardNetwork, 
	merchants MerchantLookup, 
	client *apiclient.Client
) *saga.Saga[PaymentWorkflow] {
	authorize := authorize{Payments: payments, CardNetwork: cardNetwork}
	capture := capture{Payments: payments, CardNetwork: cardNetwork}
	webhookStep := webhook{Merchants: merchants, Client: client}

	return saga.New(authorize, capture, webhookStep)
}
