package payment

import (
	"context"
	"errors"
)

var ErrConflict = errors.New("idempotency_conflict")

type PaymentRepository interface {
	InsertPending(ctx context.Context, p *Payment) error
	FindByIdem(ctx context.Context, merchantID, key string) (*Payment, error)
	MarkFailed(ctx context.Context, p *Payment) error
	MarkAuthorized(ctx context.Context, p *Payment) error
	MarkCaptured(ctx context.Context, p *Payment) error
	EnqueueReconciliation(ctx context.Context, paymentID, idemKey, step string, cause error) error
}

// AuditStore is the ledger side of Service. It mirrors AuditRepository's
// method set so Service depends on an interface (as it does for app) and
// package payment never imports the repositories subpackage — which
// imports payment for the Payment type — avoiding an import cycle.
type AuditStore interface {
	Insert(ctx context.Context, paymentID string, amount int64) error
	MarkFailed(ctx context.Context, id string) error
}

// CardNetwork is the external card-network seam. Every method carries the
// caller's idempotency key; implementations MUST honor it so retries and
// status queries never double-execute the remote operation.
type CardNetwork interface {
	// Authenticate holds funds for paymentID via an idempotent authorize call.
	Authenticate(ctx context.Context, paymentID string, amount int64, idemKey string) error
	// Capture settles a previously authenticated paymentID.
	Capture(ctx context.Context, paymentID string, amount int64, idemKey string) error
	// GetPayment reports the remote (id, status) for paymentID without
	// interpreting it; callers map the simulator vocabulary themselves.
	GetPayment(ctx context.Context, paymentID string) (id, status string, err error)
	// Retryable reports whether err is transient (timeout, network, 5xx).
	// Business refusals (4xx) are never retryable.
	Retryable(err error) bool
}

// Network statuses returned by CardNetwork.GetPayment. They mirror the
// simulator vocabulary decoded in the repositories implementation; keep
// both in sync.
const (
	networkAccepted = "accepted"
	networkRejected = "rejected"
)
