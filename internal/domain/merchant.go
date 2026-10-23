package domain

import (
	"context"
	"time"
)

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
	Price     Price
	ProductID string
}

type MerchantRepository interface {
	Create(ctx context.Context, m *Merchant) error
	FindByID(ctx context.Context, id string) (*Merchant, error)
	List(ctx context.Context, limit, offset int) ([]Merchant, int, error)
	Update(ctx context.Context, id string, name, email, webhook *string) (*Merchant, error)
	Delete(ctx context.Context, id string) error
}
