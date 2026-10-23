package domain

import (
	"context"
	"fmt"
)

type NetworkStatus string

// Network statuses returned by CardNetwork.GetPayment. They mirror the
// simulator vocabulary decoded in the repositories implementation; keep
// both in sync.
const (
	NetworkAccepted NetworkStatus = "accepted"
	NetworkRejected NetworkStatus = "rejected"
	NetworkCaptured NetworkStatus = "captured"
)

// CardNetwork is the external card-network seam. Every method carries the
// caller's idempotency key; implementations MUST honor it so retries and
// status queries never double-execute the remote operation.
type CardNetwork interface {
	// Authenticate holds funds for paymentID via an idempotent authorize call.
	Authorize(ctx context.Context, paymentID string, amount int64, idemKey string) (authorizeKey string, err error)
	// Capture settles a previously authenticated paymentID.
	Capture(ctx context.Context, authorizeKey string) error
	// GetPayment reports the remote (id, status) for paymentID without
	// interpreting it; callers map the simulator vocabulary themselves.
	GetPayment(ctx context.Context, paymentID string) (status NetworkStatus, err error)
}

type ErrorCardNetwork struct {
	Path string
	Err  error
}

func (e ErrorCardNetwork) Error() string {
	return fmt.Sprintf("card network %s: %v", e.Path, e.Err)
}

func (e ErrorCardNetwork) Unwrap() error {
	return e.Err
}
