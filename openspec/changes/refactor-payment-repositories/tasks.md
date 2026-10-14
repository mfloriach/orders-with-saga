# Tasks

## 1. New `repositories` package

- [x] 1.1 Create `internal/payment/repositories/payments.go` (`package repositories`) with `PaymentDatabaseRepository` + `NewPaymentDatabaseRepository`, same method set as `AppRepository`, importing `payment` for the `Payment` type, and verify `gofmt -l` is clean on the new file
- [x] 1.2 Create `internal/payment/repositories/audit.go` (`package repositories`) with `AuditRepository` + `NewAuditRepository` (same methods as `LedgerRepository`) and verify `gofmt -l` is clean on the new file
- [x] 1.3 Delete `internal/payment/repository_payments.go` and `internal/payment/repository_ledge.go` and verify `grep -rn "type AppRepository\|type LedgerRepository\|func NewAppRepository\|func NewLedgerRepository" internal/ cmd/ 2>/dev/null` returns nothing

## 2. Wiring (`payment` + `app`)

- [x] 2.1 Add `AuditStore` interface (`Insert`, `MarkFailed`) to `package payment`, change `Service.ledger` field and `NewService` to take it, and verify `go build ./internal/payment/` passes
- [x] 2.2 Update `internal/app/container.go` to construct `repositories.NewPaymentDatabaseRepository(appDB)` / `repositories.NewAuditRepository(ledgerDB)` and pass them to `payment.NewService`, and verify `go build ./...` passes

## 3. Tests move + fix

- [x] 3.1 Create `internal/payment/repositories/payments_test.go` (`package repositories`) merging `TestMarkAuthorizedRoundTrip` and `TestEnqueueReconciliationRoundTrip` with a single `openTestDB` helper (+ `seedPending`), updated to the new constructors, and verify `gofmt -l` is clean on it
- [x] 3.2 Delete `internal/payment/authorized_test.go` and `internal/payment/outbox_test.go` and verify no `package payment` test still references `NewAppRepository`/`NewLedgerRepository` via grep

## 4. Verification

- [x] 4.1 Run `go vet ./...` and verify it exits 0
- [x] 4.2 Run `go test ./internal/payment/...` (DB tests skip without `APP_DSN`; rest must pass) and verify exit 0
- [x] 4.3 Run final `grep -rn "AppRepository\|LedgerRepository" --include='*.go' internal/ | grep -v openspec` and verify only `PaymentDatabaseRepository` / `AuditRepository` / `AuditStore` remain

## 5. MasterCard move

- [x] 5.1 Move `repository_card_network_master_card.go` to `internal/payment/repositories/mastercard.go` (`package repositories`, `var _ payment.Processor` assertion), update `container.go` to `repositories.NewMasterCardRepository` / `repositories.MasterCardConfig`, delete the old file, and verify `go build ./...` passes
- [x] 5.2 Move `mastercard_test.go` to `internal/payment/repositories/mastercard_test.go` (`package repositories`), delete the old file, and verify `go test ./internal/payment/...` exits 0
- [x] 5.3 Run `go vet ./...` and `grep -rn "payment\.NewMasterCardRepository\|payment\.MasterCardConfig" --include='*.go' internal/` (expect no matches) and verify both pass

## 6. Status enum

- [x] 6.1 Change `model.go` to `type Status string` with typed constants, `Payment.Status Status`, `allowedTransitions map[Status][]Status`, `TransitionTo(to Status)`, and verify `go build ./internal/payment/` passes
- [x] 6.2 Update `repositories/payments.go` (`p.Status = payment.StatusPending`, scan status into a string var then convert) and `repositories/payments_test.go` status comparison, and verify `go build ./...` passes
- [x] 6.3 Update `model_test.go` tables to `[][2]Status` (with `Status("bogus")` / `Status("")` cases), `processor_test.go` `wantStatus` field to `Status`, `handler_test.go` comparison to `string(StatusAuthorized)`, and verify `go test ./internal/payment/...` exits 0

## 7. CardNetwork seam

- [x] 7.1 Replace `processor.go` with `card_network.go` (`CardNetwork`: `Authenticate`/`Capture`/`GetPayment`/`Retryable`), rewire `service.go` `callProcessor` + `authorize`/`capture` helpers (field/param `processor` → `network`), update `container.go` return type, and verify `go build ./...` passes
- [x] 7.2 Rewrite `repositories/mastercard.go` (`Authenticate`/`Capture` via `POST /payment/{id}/capture`/`GetPayment` raw passthrough, remove `Call`/`Status`/`CreatePayment`, assert `payment.CardNetwork`) and verify `go build ./...` passes
- [x] 7.3 Rewrite `repositories/mastercard_test.go` for the new methods (auth PUT/retries/refused/timeout, capture POST success/refused, GetPayment passthrough/errors, Retryable matrix) and verify `go test ./internal/payment/repositories/` exits 0
- [x] 7.4 Rewrite `processor_test.go` stub + cases for the new flow (auth+capture success, terminal auth fail, reconcile-accepted with capture ok/refused/retryable, rejected, query-down) and verify `go test ./internal/payment/` exits 0
- [x] 7.5 Run `go vet ./...`, `go test ./...`, and `grep -rn "Processor\b\|\.Call(\|\.Status(" --include='*.go' internal/ | grep -v CardNetwork | grep -v StatusCode` and verify clean
