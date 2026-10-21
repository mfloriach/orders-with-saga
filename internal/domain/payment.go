package domain

import (
	"errors"
	"time"
)

var ErrConflict = errors.New("idempotency_conflict")

// Price is money denominated in a coin. Amount is in minor units; Coin is the
// currency (expected ISO 4217, e.g. "USD").
type Price struct {
	Amount int64  `json:"amount" validate:"required,gt=0"`
	Coin   string `json:"coin" validate:"required,oneof=JPY USD EUR"`
}

type Payment struct {
	ID             string    `json:"id"`
	MerchantID     string    `json:"merchant_id"`
	Price          Price     `json:"price"`
	IdempotencyKey string    `json:"idempotency_key"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type PaymentPending struct {
	ID             string    `json:"id"`
	MerchantID     string    `json:"merchant_id"`
	Price          Price     `json:"price"`
	IdempotencyKey string    `json:"idempotency_key"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type PaymentAuthenticated struct {
	PaymentPending
	AuthenticationKey string
}

type PaymentCaptured struct {
	PaymentAuthenticated
	AuthenticationKey *string
	PaidAt            time.Time
}

type PaymentFailed struct {
	PaymentPending
	Reason string
}

// Status is the payment lifecycle state. A distinct type (not a bare
// string) so only declared statuses compile — an undeclared value cannot
// reach TransitionTo or the store by accident. The database CHECK
// constraint mirrors this vocabulary; extend both together.
type Status string

// Payment statuses. The database CHECK constraint mirrors this vocabulary;
// extend both together.
const (
	StatusPending    Status = "pending"
	StatusAuthorized Status = "authorized"
	StatusCaptured   Status = "captured"
	StatusFailed     Status = "failed"
)
