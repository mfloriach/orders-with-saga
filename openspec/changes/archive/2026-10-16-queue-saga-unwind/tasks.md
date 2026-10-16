# Tasks

## 1. Intent record and message

- [x] 1.1 Add migration `005_compensation_outbox.sql` (`payment_id` PK refs payments, `saga_id`, `completed TEXT[]` ordered, `failed_step`, `step_error`, `claimed_at NULL`, `created_at`) and verify `migrate up` applies cleanly on a scratch DB.
- [x] 1.2 Add `compensation.Intent` type (JSON shape from design decision 2) plus repo methods `RecordIntent`/`ClaimIntent`/`DeleteIntent` in `internal/payment/repositories`, and verify `go test ./internal/payment/repositories/` passes with round-trip coverage.
- [x] 1.3 Add compensation topology constants (`saga.compensate` route/queue/DLQ) and declare them in `internal/app/rabbitmq.go` alongside the webhook/email topology, and verify broker boot declares the queues (existing container test or `go vet ./internal/app/`).

## 2. Saga returns the plan instead of unwinding

- [x] 2.1 Change `pkg/saga` so `Run` records intent data on `RunError` (completed names in order) and no longer spawns the unwind goroutine; keep the in-doubt early return unchanged, and verify `go test ./pkg/saga/` passes (update async-unwind tests to assert the returned plan).
- [x] 2.2 Update `payment.Service.Create` to write the outbox row and publish the intent message on saga failure (publish after commit; publish failure is logged loudly, row remains source of truth), and verify `go test ./internal/payment/ -run TestCreate` passes with a fake publisher asserting one intent per failure and none on success or in-doubt.

## 3. Worker compensation consumer

- [x] 3.1 Implement the compensation consumer (new package following `internal/webhooks.Consumer` shape: `x-attempts` retry, backoff, Nack-to-DLX at cap, unknown step name goes straight to DLQ), rebuilding steps from the shared `aa` registry and running recorded names in reverse, and verify consume-order, retry-exhaustion-to-DLQ, and terminal-mark behavior with unit tests using fakes.
- [x] 3.2 Harden each step `Compensate` (`insert-payment`, `authorize`, `call-api-capture`) to treat already-applied state as success, and verify a replay test matrix (each compensation run twice against applied state asserts success with no second side effect) passes.
- [x] 3.3 Wire the consumer into `cmd/worker/main.go` (own channel, errCh slot) and verify `go build ./...` plus worker boot against a local broker consumes a seeded intent end to end.

## 4. Integration verification

- [x] 4.1 Run the full suite `go test ./...` and `go vet ./...` green, and verify a manual failure injection (failing card-network stub) leaves one outbox row, one queued message, a 500 to the caller, and — after the worker runs — the terminal payment state with the intent acknowledged.
- [x] 4.2 Update the saga-library spec delta if implementation surfaced wording drift, and verify `openspec validate queue-saga-unwind` passes.
