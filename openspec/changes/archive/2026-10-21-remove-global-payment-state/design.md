# Design

## Context

`internal/payment/repositories/payments.go` exposes `FindByIdem` returning
`*domain.Payment`, a flat struct that conflates every lifecycle state
(`pending`, `authorized`, `captured`, `failed`). `Captured` and `Failed` are
stubs that never persist transitions, so a successful creation returns an empty
`PaymentCaptured` and failures never mark a row. See proposal.md - Why.

`domain.Status` vocabulary and the DB CHECK constraint already cover
`pending/authorized/captured/failed`; `payment-processing` already specifies
the legal transitions (`pending→authorized`, `pending→failed`,
`authorized→failed`, `authorized→captured`) and rejects all others before a
write. No migration or status-vocabulary change is needed.

## Goals / Non-Goals

**Goals:**
- Remove `domain.Payment`; `FindByIdem` returns `domain.PaymentPending`
  carrying the live DB status.
- Implement `Captured` (`authorized → captured`) and `Failed`
  (`pending → failed` / `authorized → failed`) with `WHERE status = <expected>`
  guards and rows-affected checks, matching `Authorized`'s pattern.
- Fail fast on illegal transitions with no DB write.

**Non-Goals:**
- Adding, removing, or renaming statuses.
- Changing the `POST /payments` API contract or idempotency behavior.
- Touching migrations, `domain.Status`, or other specs.

## Decisions

**1. `FindByIdem` returns `*domain.PaymentPending` instead of `*domain.Payment`.**
- Rationale: `PaymentPending` carries every field `FindByIdem` scans
  (`ID`, `MerchantID`, `Price`, `IdempotencyKey`, `Status`, `CreatedAt`) and
  is already the state-specific type for a freshly-created payment. It avoids
  resurrecting a global catch-all.
- Alternative considered: return an interface `domain.Payment` satisfied by
  all state types. Rejected — adds indirection for no benefit; callers only
  need `ID`, `IdempotencyKey`, and `Status` for the conflict path.
- Impact: `domain.PaymentRepository`, `ConflictError.Existing`, `service.go`,
  `handler.go`, `container.go` all change types. **BREAKING**.

**2. `Captured` and `Failed` persist transitions with a status guard.**
- Rationale: mirrors `Authorized`'s `UPDATE ... WHERE status = <expected>`
  plus `RowsAffected() == 0` check, so an illegal transition (e.g.
  `captured → pending`) fails before writing and returns a clear error.
- `Captured`: `WHERE status = 'authorized'` → `status='captured'`, populate
  `PaymentCaptured` (embed input, `AuthenticationKey *string`, `PaidAt`).
- `Failed`: `WHERE status IN ('pending','authorized')` → `status='failed'`,
  populate `PaymentFailed` with `Reason`.

**3. Keep `New` and `Authorized` unchanged.**
- They already persist `pending` and `pending → authorized` correctly.

## Risks / Trade-offs

[Callers that type-assert `*domain.Payment` from `FindByIdem`/`ConflictError`
will break at compile time] → Update `service.go`, `handler.go`, `container.go`
in the same change; no runtime risk since the type change is compile-checked.

[`Failed` accepts both `pending` and `authorized`] → A payment already failed
can't be failed twice (`WHERE status IN ('pending','authorized')` excludes
`'failed'`), so re-failing is a no-op error, not a double-write.

[Empty `AuthenticationKey` for captured rows] → `PaymentCaptured` embeds
`PaymentAuthenticated.AuthenticationKey` as a non-pointer `string` while
`PaymentCaptured.AuthenticationKey` is `*string`; populate carefully to avoid
shadowing. Documented in tasks.

## Migration Plan

- Apply the type changes across domain/repositories/service/handler/container
  in one commit.
- `go build ./...` and `go test ./...` as the rollback gate; revert the single
  change if either fails. No DB migration, no data backfill.
