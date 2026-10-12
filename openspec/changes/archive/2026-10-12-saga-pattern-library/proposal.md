# Proposal

## Why

Payment creation spans two stores (app DB `payments` row + ledger DB entry) with hand-rolled compensation (`MarkFailed` on ledger failure in `internal/payment/service.go`). Every new multi-step flow would re-implement step ordering, reverse unwind, error aggregation, and tracing/logging. A small shared saga library in `pkg/` removes that duplication now, while carrying the fields a future forward-recovery sweeper will need.

## What Changes

- Add `pkg/saga` sequential orchestrator: `Saga{ID, Name, Steps}`, `Step{Name, Do, Compensate}`, `Run(ctx) error` returning `*RunError`.
- Backward compensation only: on step failure, run completed steps' compensations in reverse order, best-effort (collect errors, continue unwinding).
- Wired observability: one otel span per saga + per step, `slog` records on step failure and compensation outcome; logger/tracer injectable via options, nil-safe for tests.
- Sequential execution only; no parallel branches, no retry policy, no durable saga log in this change.
- Forward-compatibility fields only for reconciliation: stable saga/step names, saga ID (uuid default), `RunError{FailedStep, StepErr, CompensateErrs, Completed}` so a later sweeper can persist and resume. No worker or cron in this change.
- Adopt in `internal/payment.Service.Create` as first consumer (2 steps: insert pending + ledger insert, compensate = mark failed). No externally visible behavior change.

## Capabilities

### New Capabilities

- `saga-library`: sequential saga orchestration with reverse compensation, aggregated errors, and per-step observability; forward-compatible result shape for future reconciliation.

### Modified Capabilities

None. Payment-processing behavior (pending/failed/500 semantics) is unchanged; only the implementation mechanism moves onto the library.

## Impact

- New code: `pkg/saga/` (one source file + tests). Stdlib + already-present `go.opentelemetry.io/otel` and `log/slog` only; no new module dependencies.
- Modified: `internal/payment/service.go` to build steps and call `saga.Run` instead of inline if-err compensation.
- No API, migration, or config changes.
