package steps

import (
	"paymentgateway/internal/domain"
	"paymentgateway/internal/webhooks"
	"paymentgateway/pkg/saga"
)

// Registry builds the create-payment step set. The service and the
// compensation worker share this constructor so the two can never drift:
// the same constructors feed the saga and the unwind lookup below.
func Registry(payments domain.PaymentRepository, network domain.CardNetwork, events webhooks.Publisher) []saga.Step[domain.Payment] {
	return []saga.Step[domain.Payment]{
		PendingPayment{Payments: payments},
		Authorize{Payments: payments, CardNetwork: network},
		Capture{Payments: payments, CardNetwork: network},
		Webhook{Publisher: events},
	}
}
