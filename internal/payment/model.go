package payment

import (
	"fmt"
	"time"
)

type Payment struct {
	ID             string    `json:"id"`
	MerchantID     string    `json:"merchant_id"`
	Amount         int64     `json:"amount"`
	IdempotencyKey string    `json:"idempotency_key"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// Payment statuses. The database CHECK constraint mirrors this vocabulary;
// extend both together.
const (
	StatusPending    = "pending"
	StatusAuthorized = "authorized"
	StatusCaptured   = "captured"
	StatusFailed     = "failed"
)

// allowedTransitions maps a current status to its permitted targets.
// Terminal states map to nothing; unknown states fail closed.
var allowedTransitions = map[string][]string{
	StatusPending:    {StatusAuthorized, StatusFailed},
	StatusAuthorized: {StatusFailed, StatusCaptured},
	StatusFailed:     {},
	StatusCaptured:   {},
}

// TransitionTo rejects an illegal or unknown move from the payment's
// current status before any database write occurs.
func (p *Payment) TransitionTo(to string) error {
	for _, next := range allowedTransitions[p.Status] {
		if next == to {
			return nil
		}
	}
	return fmt.Errorf("payment: illegal transition %q -> %q", p.Status, to)
}
