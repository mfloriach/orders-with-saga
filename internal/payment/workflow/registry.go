package workflow

import (
	"paymentgateway/internal/domain"
	"paymentgateway/internal/notifications/email"
	"paymentgateway/pkg/apiclient"
	saga "paymentgateway/pkg/saga"
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
	client *apiclient.Client,
	emailSender email.Senders,
) *saga.Saga[PaymentWorkflow] {
	authorize := authorize{Payments: payments, CardNetwork: cardNetwork}
	capture := capture{Payments: payments, CardNetwork: cardNetwork}
	webhookStep := webhook{Client: client}
	emailStep := emailStep{Sender: emailSender}

	return saga.New([]saga.Step[PaymentWorkflow]{authorize, capture, webhookStep, emailStep})
}
