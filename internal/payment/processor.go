package payment

import "context"

// Processor is the external payment API seam. Every method carries the
// caller's idempotency key; implementations MUST honor it so retries and
// status queries never double-execute the remote operation.
type Processor interface {
	// Call attempts the remote operation for paymentID.
	Call(ctx context.Context, paymentID string, amount int64, idemKey string) error
	// Status reports whether the remote accepted the operation with idemKey.
	Status(ctx context.Context, idemKey string) (accepted bool, err error)
	// Retryable reports whether err is transient (timeout, network, 5xx).
	// Business refusals (4xx) are never retryable.
	Retryable(err error) bool
}
