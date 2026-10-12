# Tasks

## 1. Saga in-doubt signal

- [x] 1.1 Add in-doubt skip-unwind to `pkg/saga` (typed in-doubt error recognized via `errors.As`, `Run` skips `unwind` and returns flagged `RunError` with `Completed` intact, ordinary errors unwind as before) and verify `go test ./pkg/saga/ -v` passes
- [x] 1.2 Cover in-doubt with tests (in-doubt after completions runs zero compensations, ordinary error still unwinds in reverse) and verify the new tests pass in `go test ./pkg/saga/ -run TestInDoubt -v`

## 2. Outbox storage

- [x] 2.1 Add `migrations/app/003_reconciliation_outbox.sql` (`reconciliation_outbox` with payment_id PK/FK, idempotency_key, step, error, attempts, created_at) and verify it applies cleanly to a scratch Postgres (`psql -f`)
- [x] 2.2 Add outbox insert method to the payment app repository with a round-trip test and verify `go test ./internal/payment/ -v` passes

## 3. Processor seam and service step

- [x] 3.1 Add `Processor` interface (`Call` + `Status` keyed by idempotency key, retryable-error classifier) and inject into `Service`, and verify `go build ./...` passes
- [x] 3.2 Add last `call-api` saga step (bounded retries on transient failures only, sync status-query reconcile, in-doubt error when unconfirmed, ordinary error on confirmed failure) with stubbed-processor unit tests for all four outcomes, and verify `go test ./internal/payment/ -v` passes
- [x] 3.3 Wire in-doubt handling in `Create` (skip unwind, keep `pending`, insert outbox row, return error → existing `500`; clear failure keeps mark-failed path) and verify `go test ./...` and `go vet ./...` pass with handler status codes unchanged
