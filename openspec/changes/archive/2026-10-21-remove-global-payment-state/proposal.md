# Proposal

## Why

The domain layer exposes a single flat `domain.Payment` struct used as the
catch-all return type for `FindByIdem` and the idempotency conflict path. It is
"too global" — it carries no state-specific meaning and conflates every
lifecycle state (`pending`, `authorized`, `captured`, `failed`) into one shape,
while the repository's `Captured` and `Failed` methods are stubs that never
persist the status transitions the `payment-processing` spec already requires.

## What Changes

- Remove `domain.Payment` (the global catch-all struct). **BREAKING**.
- `FindByIdem` returns `domain.PaymentPending` instead of `*domain.Payment`,
  scanning the live `status` column so callers see the payment in its current
  state.
- Implement `Captured` (`authorized → captured`) and `Failed`
  (`pending → failed` / `authorized → failed`) to persist status transitions,
  guarded with `WHERE status = <expected>` like `Authorized`, failing fast on
  illegal transitions.
- No change to `domain.Status` vocabulary, the DB CHECK constraint, migrations,
  or the `POST /payments` API contract.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This is a pure refactor plus implementation of transitions already
specified by `payment-processing`; no spec-level requirement changes, so no
delta spec is needed.

## Impact

- `internal/domain/payment.go` — remove `Payment`
- `internal/domain/interfaces.go` — `FindByIdem` return type, `ConflictError.Existing`
- `internal/payment/repositories/payments.go` — `FindByIdem`, `Captured`, `Failed`
- `internal/payment/service.go` — `FindByIdem` result usage
- `internal/payment/handler.go` — conflict path reads `Existing`
- `internal/app/container.go` — type wiring for `domain.PaymentRepository`
