# Proposal

## Why

`PaymentRepository.MarkFailed/MarkAuthorized/MarkCaptured` take a bare `id string` while the caller in `service.go` also owns the in-memory transition (`TransitionTo` + a stray `p.Status =` assignment). Every status change is written twice in two places with two vocabularies, and on DB failure `p` claims a status never persisted.

## What Changes

- **BREAKING** `PaymentRepository`: `MarkFailed(ctx, id string)` → `MarkFailed(ctx, p *Payment)`, same for `MarkAuthorized` / `MarkCaptured`. Each method assigns `p.Status` (mutate-then-write) and persists with the existing preconditioned `UPDATE ... WHERE status = <expected>`; no `TransitionTo` call inside the repo — the SQL `WHERE` clause is the single guard.
- `service.go`: drop the `TransitionTo` calls and the stray `p.Status = StatusCaptured` at the `MarkCaptured` site; call sites pass `p`.
- `AuditStore.MarkFailed` stays `(ctx, id string)` — it marks a ledger entry, not a payment.
- Fix compensate `ctx` → `c` at the `MarkFailed` call site (`service.go:56`).
- Update `repositories/payments_test.go` call sites to the new signatures.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
(none — pure refactor; observable payment behavior, statuses, and transition vocabulary are unchanged)

## Impact

- `internal/payment/interfaces.go` (interface), `internal/payment/repositories/payments.go` (3 methods), `internal/payment/service.go` (4 call sites + removal of `TransitionTo` imports if unused), `internal/payment/repositories/payments_test.go`.
- No migration, no API, no spec-behavior change. `go build ./...` + `go test ./internal/payment/...` verify.
