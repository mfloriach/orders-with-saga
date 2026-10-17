package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxKeyLength bounds explicit header keys; longer values fall back to derivation.
	MaxKeyLength = 200
	// ExplicitTTL is the dedup window for client-supplied keys.
	ExplicitTTL = 24 * time.Hour
	// DerivedTTL bounds the retry-vs-coincidence collision for derived keys.
	DerivedTTL = 10 * time.Minute
)

func GenerateKey(req []byte) string {
	sum := sha256.Sum256(req)
	return "drv:" + hex.EncodeToString(sum[:])
}

// Derive mints a fresh key for a request that carried no Idempotency-Key.
// Unique per call, so keyless requests are never deduped against each other.
func Derive() string { return "drv:" + uuid.NewString() }
