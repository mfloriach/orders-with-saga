# Design

## Context

See proposal.md — Why. Current state: `internal/payment/service.go:29-56` compensates inline (`MarkFailed` on ledger failure) for exactly 2 steps. `pkg/` libraries (`apperr`, `idempotency`, `logging`, `tracing`) are tiny single-package units, stdlib-first, no single-impl interfaces. otel SDK with no exporter is already initialized (`pkg/tracing`); `pkg/logging.Handler` injects `request_id`/`trace_id` from ctx. `go.mod` already includes `go.opentelemetry.io/otel` and `google/uuid` — no new deps needed.

## Goals / Non-Goals

**Goals:**
- One `pkg/saga` orchestrator usable by payment `Create` today and future multi-step flows.
- Reverse best-effort unwind with aggregated errors; result shape a future sweeper can persist/resume.
- Wired otel + slog per step, injectable and nil-safe.

**Non-Goals:**
- Forward recovery worker/cron, durable saga log table, retries/backoff, parallel branches — explicitly deferred (see proposal).

## Decisions

**1. Plain structs over generics — `Saga{ID, Name, Steps}`, `Step{Name, Do, Compensate func(ctx) error}`.**
Steps share no payload type across flows (payment passes IDs via closure). Generics (`Saga[T]`) would force a uniform state type and complicate the payment call site for zero benefit at 2–3 steps. Closures capture per-flow state; nil `Compensate` = nothing to undo. Alternative (generic state threading) rejected: heavier API, no caller needs it yet.

**2. `Run(ctx) *RunError`-compatible error, not `(T, error)` tuple.**
Saga returns only control flow; values flow through closures. `RunError{SagaID, SagaName, FailedStep, StepErr, Completed []string, CompensateErrs []StepError}` satisfies `error` and `errors.As` so `apperr.Handler` and callers can unwrap. `Completed` in execution order gives the future sweeper its resume set. Alternative (return completed list separately) rejected: single error value keeps call sites to `if err := saga.Run(ctx); err != nil`.

**3. Best-effort reverse unwind, always attempt even on cancelled ctx.**
Loop `completed` backwards; each compensate error appended, never aborts the loop. Compensations run with the caller ctx unless already cancelled, in which case a short detached timeout ctx is used — rationale: payment must still `MarkFailed` after client disconnect, otherwise orphans accumulate. Alternative (respect cancellation strictly) rejected: trades orphan safety for ctx purity.

**4. Wired observability via injected `*slog.Logger` + global otel tracer, nil-safe.**
`Options{Logger, TracerName}`; default logger = `slog.Default()` (already wrapped by `logging.Handler` in prod), default tracer = `otel.Tracer("pkg/saga")`. One saga span, child span per `Do` and per `Compensate`; failed spans `RecordError` + `SetStatus(Error)`. Logs at `Error` for step failure, `Warn` for compensate failure, `Info`/`Debug` for success — with `saga_id`, `saga_name`, `step` attrs. Alternative (saga takes no logger, caller logs) rejected: per user decision for wired; injection keeps tests hermetic.

**5. Sequential only, enforced by absence.**
No `Concurrency`, `MaxParallel`, or `RetryPolicy` fields. Sequential `for` loop with a ctx-cancel check before each `Do`. Parallelism later = new proposal with its own compensation-ordering semantics (reverse-completion vs reverse-declaration), not a flag flip.

**6. First consumer: payment `Create` as 2 steps.**
Step `insert-payment` (Do=`InsertPending`, Compensate=nil — nothing persisted yet on its own failure... on later failure the payment row exists so compensate=`MarkFailed`), step `insert-ledger` (Do=`ledger.Insert`, Compensate=`MarkFailed`). Preserves current 500/failed semantics byte-for-byte; deletes the inline if-err branch. Validates the API against a real caller before other flows adopt it.

## Risks / Trade-offs

- [Compensate itself fails (e.g. DB down during `MarkFailed`)] → aggregated into `RunError.CompensateErrs`; caller logs and returns 500; orphan remains `pending` until the future sweeper exists. Accepted: surfaces instead of hiding.
- [Detached-ctx compensate outlives request] → bounded by short timeout (e.g. 5s, configurable via Options); prevents goroutine leak.
- [Step names collide/rename breaks future resume] → document names as stable identifiers; validation rejects empty/duplicate step names at `Run` time (fail-fast, no partial execution).
- [Span explosion if sagas nest] → nesting unsupported in this change; a `Do` that calls `Run` creates nested spans but flat compensation — documented as unsupported.

## Migration Plan

1. Land `pkg/saga` + unit tests; no callers yet (safe).
2. Refactor `internal/payment.Service.Create` onto saga; existing handler tests (`handler_test.go`) + payment flow cover regression; no migration, no config change.
3. Rollback = revert the service commit; library unused is inert.

## Open Questions

None that change specs, approach, or tasks. Deferrable tuning (detached-ctx timeout value, success-log level) resolves in review.
