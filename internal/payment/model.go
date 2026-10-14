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
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
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

// allowedTransitions maps a current status to its permitted targets.
// Terminal states map to nothing; unknown states fail closed.
var allowedTransitions = map[Status][]Status{
	StatusPending:    {StatusAuthorized, StatusFailed},
	StatusAuthorized: {StatusFailed, StatusCaptured},
	StatusFailed:     {},
	StatusCaptured:   {},
}

// TransitionTo rejects an illegal or unknown move from the payment's
// current status before any database write occurs.
func (p *Payment) TransitionTo(to Status) error {
	for _, next := range allowedTransitions[p.Status] {
		if next == to {
			p.Status = to
			return nil
		}
	}
	return fmt.Errorf("payment: illegal transition %q -> %q", p.Status, to)
}
