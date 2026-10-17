# Tasks

## 1. Terminal-state table test

- [x] 1.1 Add `internal/payment/steps/terminal_test.go` with test-local fakes (`domain.PaymentRepository` with status guards, `domain.CardNetwork` with canned answers, `webhooks.Publisher` with error toggle) and verify `go vet ./internal/payment/steps/` passes.
- [x] 1.2 Add the unwind harness (build `Registry`, run `saga.New(...).Run`, reverse-run `Compensate` over `RunError.Completed`, read row via `FindByIdem`) plus rows: insert failure → untouched pending, authorize refused → failed, and verify the new test passes.
- [x] 1.3 Add capture rows (network timeout → in-doubt/pending with no unwind; `MarkCaptured` error with remote rejected and accepted → failed) and webhook row (publish error → failed), and verify `go test ./internal/payment/steps/ -v` passes fully.
- [x] 1.4 Run `go test ./...` and `go vet ./...` green and verify `openspec validate saga-terminal-state-tests` passes.
- [x] 1.5 Split `terminal_test.go` into per-step files (`insert`, `authorize`, `capture`), keeping fakes and the harness shared, and verify `go test ./internal/payment/steps/ -v` passes.
