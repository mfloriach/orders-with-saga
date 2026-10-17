# Tasks

## 1. Saga observer hook

- [x] 1.1 Add `OnStep` field and `WithOnStep[T]` option to `pkg/saga`, invoke it synchronously after each executed `Do` (skipped when cancellation preempts the step), and verify `go test ./pkg/saga/ -count=1` passes with a hook-order test (success + failure + unexecuted-step silence).
- [x] 1.2 Verify the hook cannot affect the saga outcome (nil hook, hook that panics is out of scope — document that observers must not panic) via the test in 1.1 and `go vet ./pkg/saga/`.

## 2. Service audit wiring

- [x] 2.1 Build the saga per `Create` with `WithOnStep` recording `StepRecord{PaymentID, SagaID, Step, Outcome, Reason}` via a new `audit domain.AuditStore` field; audit-write failure logs loudly without failing the payment, and verify `go test ./internal/payment/ -count=1` passes with a fake `AuditStore` (one row per executed step, failure row carries reason, write error spares payment).
- [x] 2.2 Thread the seam: add the `audit` param to `payment.NewService`, pass the existing `AuditRepository` in `internal/app/container.go`, update `service_test.go` constructors, and verify `go build ./...` passes.
- [x] 2.3 Run `go test ./...` and `go vet ./...` green and verify `openspec validate saga-step-audit` passes.
