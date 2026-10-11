# Proposal

## Why

The idempotency key lives in the request body, forcing body parsing before the key is known and mixing transport concerns with domain data. Moving it to the `Idempotency-Key` header follows the industry standard (Stripe-style), keeps the body purely `merchant_id` + `amount`, and makes key handling uniform. Not in production, so this is a hard cut with no transition.

## What Changes

- **BREAKING**: `POST /payments` drops the `idempotency_key` body field; the key travels in the `Idempotency-Key` request header instead.
- Missing header no longer fails: the server derives a key as `drv:` + hex `SHA-256(merchant_id:amount)` so plain retries stay idempotent.
- Derived keys get a short window (~10 min) to bound the retry-vs-coincidence collision; explicit client keys keep 24h.
- Responses echo the effective key in the `Idempotency-Key` response header so clients can see when a key was derived.
- Key resolution lives in the payment service (`ResolveKey` on the raw header); the handler passes the header through and echoes the effective key from the payment object.

## Capabilities

### New Capabilities

None — idempotency behavior already exists; this change moves its transport.

### Modified Capabilities

- `payment-processing`: "Create payment" (key source changes) and "Payment idempotency" (header source, derivation, dual windows) requirements change.

## Impact

- `internal/payment/handler.go` (header read, `ResolveKey`, echo header, body struct shrinks), `internal/payment/service.go` (per-key TTL), `internal/payment/handler_test.go`, `migrations/app` (nothing — `idempotency_key` column unchanged, stores effective key).
- Clients must send the header; body keys are ignored. No DB, route, or infra changes.
