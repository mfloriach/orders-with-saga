# Design

## Context

See proposal.md for motivation. Current state (`internal/payment`, all `package payment`):
- `repository_payments.go` defines `PaymentRepository` interface + `AppRepository` struct; `repository_ledge.go` defines `LedgerRepository`; `repository_card_network_master_card.go` defines `MasterCardRepository`.
- `service.go:20-27` takes `app PaymentRepository` (interface) + `ledger *LedgerRepository` (concrete); `container.go:91-93` wires `payment.NewAppRepository(appDB)` / `payment.NewLedgerRepository(ledgerDB)`.
- `repositories/` exists but is empty. `authorized_test.go` + `outbox_test.go` both exercise `NewAppRepository` against a migrated app DB (`APP_DSN`, skip-if-absent); `outbox_test.go` inline-duplicates the `openTestDB` helper from `authorized_test.go`.

Constraint: a Go directory is a package boundary, so `repositories/` must become `package repositories` — a same-named `package payment` in two dirs would be two distinct packages and maximally confusing.

## Goals / Non-Goals

**Goals:**
- DB repos live in `package repositories` with honest names; `payment` keeps domain + orchestration.
- Zero behavior change; `go build ./...` + `go vet` + existing tests green.

**Non-Goals:**
- Moving the `Payment` model or `PaymentRepository` interface — interface/impl split stays as described below.
- Fixing the known `ledger_entries.status` drift (no `status` column in `migrations/ledger/001_entries.sql`) — flagged, not fixed here.
- Changing `service.ledger` from concrete to interface (tempting, but expands scope; noted as follow-up).

## Decisions

1. **Interface stays in `payment`, impls move to `repositories` (over moving the model down).**
   `repositories` imports `paymentgateway/internal/payment` for the `Payment` type only; `payment` never imports `repositories` at the type level — `service.go` keeps depending on the locally-defined `PaymentRepository` interface, and only `container.go` (package `app`) imports both to wire them. Alternative (move `Payment` + interface down) is purer but drags model, status constants, and transition tests into the move for no behavior gain. Rejected as larger diff.
   - `AuditRepository` takes `*sql.DB` (ledger DB); `PaymentDatabaseRepository` takes `*sql.DB` (app DB). Constructors: `NewAuditRepository`, `NewPaymentDatabaseRepository`.

2. **File mapping: `repository_payments.go` → `repositories/payments.go`, `repository_ledge.go` → `repositories/audit.go`, `repository_card_network_master_card.go` → `repositories/mastercard.go`.**
   Drops the redundant `repository_` prefix inside a `repositories` dir and fixes the `ledge` typo. `MasterCardRepository`/`MasterCardConfig` move unchanged; the `Processor` interface stays in `payment` and `repositories` asserts `var _ payment.Processor = (*MasterCardRepository)(nil)` — same one-directional pattern as Decision 1, no new cycle (`payment` never imports `repositories`).

3. **Tests merge into `repositories/payments_test.go` (`package repositories`).**
   Merge `TestMarkAuthorizedRoundTrip` + `TestEnqueueReconciliationRoundTrip`, keep one `openTestDB` helper (the `authorized_test.go` form with `t.Cleanup`), delete the inline duplicate in `outbox_test.go`. Originals deleted so helpers (`seedPending`) don't collide across packages. Tests keep the `APP_DSN`-skip pattern — no new hermetic fakes.

4. **`service.ledger` field type becomes `*repositories.AuditRepository`.**
   Requires `service.go` to import the subpackage — one-directional (`payment` → `repositories` for the concrete type + `payment` type used by `repositories` is the reverse edge). Wait — that's a cycle: `repositories` imports `payment` (for `Payment`), `payment` imports `repositories` (for `AuditRepository`). Two options: (a) widen `service.ledger` to a narrow `AuditStore` interface defined in `payment` (`Insert` + `MarkFailed`), implemented by `repositories.AuditRepository` — no import of `repositories` from `payment`, cycle broken, matches how `app` already uses an interface; (b) move the model down. Choose (a): define `type AuditStore interface { Insert(ctx, paymentID, amount) error; MarkFailed(ctx, id) error }` in `payment`, change `Service.ledger` + `NewService` to take it. `container.go` passes `*repositories.AuditRepository` which satisfies it implicitly.

6. **`CardNetwork` replaces `Processor`; `Capture` runs only in the reconciled branch.**
   `processor.go` → `card_network.go` with `Authenticate(ctx, paymentID, amount, idemKey) error`, `Capture(ctx, paymentID, amount, idemKey) error`, `GetPayment(ctx, paymentID) (id, status string, err error)`, `Retryable(err) bool`; `Processor` deleted. `MasterCardRepository`: `Authenticate` is today's `Call` body unchanged; `Capture` is `POST {base}/payment/{id}/capture` with the same `{"amount","idempotency_key"}` body convention, non-2xx → `statusError`, decode-and-discard, same timeout/policy; `GetPayment` is today's `Status` GET without the accepted/rejected mapping (raw passthrough); `CreatePayment` removed (unused by `Create`). `Service.callProcessor`: clear `Authenticate` success → authorize locally, return (`authorized`, per the "Authorized payment state" spec — the happy path gains no network call); terminal auth failure → return; retryable → `GetPayment`: query error → in-doubt, `rejected` → did-not-accept error, unconfirmed → in-doubt, `accepted` → authorize locally then `Capture`: success → mark `captured`, terminal refusal → error (compensates), retryable → in-doubt (capture state unknowable from a payment-status query, so no second query). `Service.processor` field + `NewService` param become `network CardNetwork`; `container.go` return type follows. Accepted/rejected literals live as unexported constants in `payment` (mirroring the simulator vocabulary in `repositories`; comment cross-references both) because `payment` cannot import `repositories` (cycle). Spec terminal states unchanged, so `skip_specs` stands; pre-existing drift noted below is untouched.
5. **`Status` becomes `type Status string`; `TransitionTo` takes `Status`.**
   Constants become typed (`StatusPending Status = "pending"` etc.), `Payment.Status` is `Status`, `allowedTransitions` is `map[Status][]Status`. Callers already use the constants (`service.go` needs no change). DB rows scan into a `string` var then convert (`payment.Status(s)`) rather than scanning into `*Status` directly — avoids relying on driver named-type behavior, and the round-trip tests skip without `APP_DSN` so the conversion must be correct by construction. JSON is untouched (underlying string marshals identically).

## Risks / Trade-offs

- [Risk] Import cycle if `service.go` imports `repositories` directly → Mitigation: `AuditStore` interface in `payment` (Decision 4); verify with `go build ./...` before touching tests.
- [Risk] Stale references to old constructors (`authorized/outbox` tests, `container.go`, docs) → Mitigation: `grep -rn "AppRepository\|LedgerRepository\|NewAppRepository\|NewLedgerRepository"` must return only historical mentions (e.g. archived openspec docs) after the change.
- [Risk] `AuditRepository.MarkFailed` still targets a `status` column migrations don't define → Trade-off accepted: out of scope, behavior unchanged; flagged in proposal non-goals.
