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

type PaymentPending struct {
	ID             string        `json:"id"`
	MerchantID     string        `json:"merchant_id"`
	Price          Price         `json:"price"`
	IdempotencyKey string        `json:"idempotency_key"`
	Status         PaymentStatus `json:"status"`
	CreatedAt      time.Time     `json:"created_at"`
}

type PaymentAuthenticated struct {
	PaymentPending
	AuthenticationKey string
}

type PaymentCaptured struct {
	PaymentAuthenticated
	PaidAt time.Time
}

type PaymentFailed struct {
	PaymentPending
	Reason string
}

type PaymentRefund struct {
	PaymentCaptured
	Reason   string
	RefundAt time.Time
}

// Status is the payment lifecycle state. A distinct type (not a bare
// string) so only declared statuses compile — an undeclared value cannot
// reach TransitionTo or the store by accident. The database CHECK
// constraint mirrors this vocabulary; extend both together.
type PaymentStatus string

// Payment statuses. The database CHECK constraint mirrors this vocabulary;
// extend both together.
const (
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusAuthorized PaymentStatus = "authorized"
	PaymentStatusCaptured   PaymentStatus = "captured"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusRefunded   PaymentStatus = "refunded"
)

// OutboxStatus is the lifecycle state of a captured payment waiting for its
// post-capture processing. The database CHECK constraint mirrors this
// vocabulary; extend both together.
type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusProcessed OutboxStatus = "processed"
	OutboxStatusFailed    OutboxStatus = "failed"
)

// OutboxPayment records a captured payment whose follow-up processing has not
// completed yet. Status pending means captured-but-unprocessed; processed and
// failed mean an attempt finished (processed = follow-up delivered, failed =
// retry pending).
type OutboxPayment struct {
	ID          string       `json:"id"`
	PaymentID   string       `json:"payment_id"`
	MerchantID  string       `json:"merchant_id"`
	Status      OutboxStatus `json:"status"`
	Attempts    int64        `json:"attempts"`
	LastError   string       `json:"last_error"`
	CreatedAt   time.Time    `json:"created_at"`
	ProcessedAt time.Time    `json:"processed_at"`
}
