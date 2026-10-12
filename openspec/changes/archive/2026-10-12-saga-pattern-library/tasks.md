# Tasks

## 1. Saga core

- [x] 1.1 Implement `pkg/saga/saga.go` (Saga/Step/Options/RunError types, sequential Run, reverse best-effort unwind, detached-timeout ctx for compensations, empty/duplicate step-name validation) and verify `go build ./...` passes
- [x] 1.2 Add `pkg/saga/saga_test.go` covering success, mid-chain failure stops forward progress, reverse order, nil-compensate skip, failed-step compensate skipped, compensate-error aggregation with continued unwind, cancelled-ctx abort, and default saga-ID generation, and verify `go test ./pkg/saga/ -v` passes

## 2. Observability

- [x] 2.1 Wire otel spans (saga span + per-step/per-compensate child spans, RecordError on failure) with injectable tracer name and verify spans assertable via oteltrace test tracer in `go test ./pkg/saga/ -run TestObservability -v`
- [x] 2.2 Wire slog records (step failure, compensate failure/success with saga_id/saga_name/step attrs, nil-safe default logger) and verify log output assertions pass in `go test ./pkg/saga/ -v`

## 3. Payment adoption

- [x] 3.1 Refactor `internal/payment.Service.Create` onto `saga.Run` (steps insert-payment/insert-ledger, MarkFailed compensations, ConflictError paths preserved) and verify `go test ./internal/payment/ -v` and `go vet ./...` pass with no behavior change to status codes
