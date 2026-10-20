package domain

import "time"

type Merchant struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	WebhookURL *string   `json:"webhook_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type MerchantEventType = string

const (
	MerchantEventTypeSuccessfullPayment = "payment.successfull"
)

type MerchantEvent struct {
	ID        string
	Type      MerchantEventType
	Merchant  Merchant
	Payload   []byte
	CreatedAt time.Time
}

type MerchantPaymentSusccessfull struct {
	PaymentID string
	Amount    int64
	ProductID string
}
