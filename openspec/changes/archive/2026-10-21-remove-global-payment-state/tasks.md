# Tasks

## 1. Domain: remove the global Payment object

- [x] 1.1 Remove `domain.Payment` from `internal/domain/payment.go`; verify nothing else in `internal/domain` references it (`go build ./internal/domain/...`)
- [x] 1.2 Change `domain.PaymentRepository.FindByIdem` to return `*domain.PaymentPending` and `ConflictError.Existing` to `*domain.PaymentPending` in `internal/domain/interfaces.go`; verify (`go build ./internal/domain/...`)

## 2. Repository: persist Captured/Failed transitions

- [x] 2.1 Confirm `FindByIdem` returns `*domain.PaymentPending` scanning live status in `internal/payment/repositories/payments.go` (already changed); verify (`go build ./internal/payment/repositories/...`)
- [x] 2.2 Implement `Captured` (`authorized → captured`) with `WHERE status='authorized'` guard, rows-affected check, populated `PaymentCaptured` (`AuthenticationKey *string`, `PaidAt`)
- [x] 2.3 Implement `Failed` (`pending → failed` / `authorized → failed`) with `WHERE status IN ('pending','authorized')` guard and `Reason`; verify (`go build ./internal/payment/repositories/...`)

## 3. Callers: service, handler, container wiring

- [x] 3.1 Update `internal/payment/service.go` for the `*domain.PaymentPending` FindByIdem result
- [x] 3.2 Update `internal/payment/handler.go` conflict path to read `Existing` as `*domain.PaymentPending`
- [x] 3.3 Update `internal/app/container.go` type wiring for `domain.PaymentRepository`; verify (`go build ./internal/...`)

## 4. Verify end to end

- [x] 4.1 `go build ./...` passes
- [x] 4.2 `go test ./...` — changed packages compile; two pre-existing unrelated failures (`internal/app` openapi.yaml init panic, `pkg/apiclient` retry test)
- [x] 4.3 Confirm no remaining references to the removed `domain.Payment` type across the repo

## Workflow follow-up

- Run `/opsx-apply` to implement the change once these tasks are reviewed.
- Archive the change after implementation and review are complete.
