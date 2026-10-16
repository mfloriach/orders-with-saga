# Tasks

## 1. Saga background unwind

- [x] 1.1 Change `pkg/saga` `Run`/`unwind` so ordinary failure returns the `RunError` immediately and compensation runs on a detached context with re-rooted spans, compensation errors logged with `saga_id`, and verify `go test ./pkg/saga/ -v` passes with async-unwind assertions
- [x] 1.2 Update saga observability tests for the new contract (no `CompensateErrs` in the return; log-based assertions) and verify `go test ./pkg/saga/ -v` passes

## 2. Audit trail

- [x] 2.1 Add the `payment_step_events` migration and `RecordStep` to `AuditStore` (plus test fakes), remove dead `StepInsertAudit`, and verify `go test ./internal/payment/...` passes
- [x] 2.2 Record every saga step outcome via a saga-level hook so future steps are covered without per-step edits, and verify a service-level test asserts one row per step plus reason on failure

## 3. Email notification

- [x] 3.1 Rename `steps.Email` to `"send-email"`, implement it as an event publisher on the webhook seam, publish after saga success in `Create`, and verify service tests assert publish-on-201 and nothing-on-failure
- [x] 3.2 Add worker email delivery (bounded retries, DLQ parking, identity logs) and verify with live-broker tests covering deliver and DLQ parking

## 4. Integration verification

- [x] 4.1 Run end-to-end against compose (201 → webhook + email delivered; dead endpoints → retries → DLQs; forced mid-flow failure → payment marked failed fast while unwind completes in background) and verify each observable step
- [x] 4.2 Run `go test ./...`, `go vet ./...`, and `openspec validate` and verify all pass
