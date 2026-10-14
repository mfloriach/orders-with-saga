# Proposal

## Why

`internal/payment/` mixes domain logic (service, model, handler) with storage implementations in a flat layout, and two repository names mislead: `LedgerRepository` suggests double-entry ledger semantics the table does not have, and `AppRepository` is a catch-all that hides it owns payments + reconciliation outbox. Renaming and grouping the DB repos makes ownership obvious before the next payment change.

## What Changes

- Move `repository_payments.go` to `internal/payment/repositories/payments.go` as `PaymentDatabaseRepository` (constructor `NewPaymentDatabaseRepository`).
- Move `repository_ledge.go` to `internal/payment/repositories/audit.go` as `AuditRepository` (constructor `NewAuditRepository`), fixing the `ledge` filename typo.
- Keep `PaymentRepository` interface and `Payment` model in `package payment`; `repositories` imports `payment` for the model type and `service.go` / `container.go` are updated to the new constructors and types.
- Move DB-backed tests (`authorized_test.go`, `outbox_test.go`) into `internal/payment/repositories/` as `package repositories`, merging the duplicated DB-open helper into one and updating constructor references.
- Move `repository_card_network_master_card.go` to `internal/payment/repositories/mastercard.go` (`MasterCardRepository`, `MasterCardConfig` unchanged) and `mastercard_test.go` to `internal/payment/repositories/mastercard_test.go` (`package repositories`); the `Processor` interface stays in `package payment` and `repositories` keeps its compile-time `payment.Processor` assertion.
- Type `Payment.Status` as a `Status` string enum (`type Status string`, typed constants) and take `Status` in `TransitionTo`, so invalid statuses fail at compile time instead of only at runtime. JSON wire format is unchanged (underlying string).
- Replace the `Processor` seam (`Call`/`Status`) with `CardNetwork` (`Authenticate`/`Capture`/`GetPayment` + `Retryable`) in `package payment`; `MasterCardRepository` implements it (`Authenticate` = today's idempotent PUT, `Capture` = `POST /payment/{id}/capture`, `GetPayment` = raw `(id, status, err)` GET). `Service` rewires to the new seam: clear auth success still ends `authorized`; reconciled-accepted authorizes then captures through the network before marking `captured`; unconfirmed capture goes in-doubt. Unused `CreatePayment` is removed.

## Capabilities

### New Capabilities

None — pure structural refactor, no new behavior.

### Modified Capabilities

None — no spec-level behavior changes. This change sets `skip_specs: true` in `.openspec.yaml`.

## Impact

- Affected code: `internal/payment/repository_*.go` (moved), `internal/payment/service.go` (field/ctor types), `internal/app/container.go` (wiring), `internal/payment/{authorized,outbox}_test.go` (moved + fixed), `internal/payment/repository_card_network_master_card.go` + `mastercard_test.go` (moved).
- **BREAKING** (Go-internal only): `payment.NewAppRepository` and `payment.NewLedgerRepository` are removed; callers must use `repositories.NewPaymentDatabaseRepository` / `repositories.NewAuditRepository`. `payment.NewMasterCardRepository` / `payment.MasterCardConfig` move to `repositories` (same names). `Payment.Status` becomes `payment.Status` (was `string`); `TransitionTo` takes `Status`. `payment.Processor` (`Call`/`Status`) is replaced by `payment.CardNetwork` (`Authenticate`/`Capture`/`GetPayment`); `MasterCardRepository.CreatePayment` is removed (unused). String literals no longer compile as statuses — use the constants. No HTTP/API or migration changes (DB CHECK constraint vocabulary and JSON encoding are unchanged); spec-level terminal states are preserved (clear success → `authorized`, reconciled-accepted → `captured`).
