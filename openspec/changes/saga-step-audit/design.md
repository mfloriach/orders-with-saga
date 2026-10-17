# Design

## Context

See proposal.md (Why). Current state: generic `saga.Saga[T]` has no step observer (the pre-generics `OnStep` was cut in the refactor); `payment.Service` builds one saga at construction (uuid shared by all payments) and holds no audit seam, though `domain.AuditStore` and `repositories.AuditRepository` already exist; container does not pass an audit repo to `NewService`.

## Goals / Non-Goals

**Goals:**
- One audit row per executed step, written synchronously with the outcome.
- Each payment gets its own saga uuid.

**Non-Goals:**
- Auditing compensations (nothing executes them in-process today).
- Retrying failed audit writes (log loudly instead).

## Decisions

**1. Generic observer hook on the saga, not a step decorator.**
`OnStep func(ctx context.Context, item *T, step string, err error)` + `WithOnStep[T]` option, invoked in the `Run` loop right after each `Do` (success or failure; skipped when cancellation preempts a step). Alternative (audit decorator around each step in the service) rejected: same per-request needs with a new wrapper type to maintain, and steps must opt in one by one.
- The hook returns nothing and cannot fail the saga (mirrors the old contract).

**2. Saga built per `Create`, not per service.**
`Create` calls `saga.New("create-payment", s.steps, saga.WithOnStep(...))` with a closure over `p` for the payment id and over the built saga for its id. Alternative (keep the shared saga, correlate by other means) rejected: no per-payment key exists on the hook path, and the shared uuid is meaningless in audit rows.

**3. Loud-log failure policy.**
`RecordStep` errors are `slog.ErrorContext` with payment id, step, and error; the payment flow is untouched. Matches the archived audit-trail behavior ("observability, not money").

## Risks / Trade-offs

- [Risk] One extra ledger INSERT per step adds latency to creation → Mitigation: same cost the specified audit trail always implied; writes stay inside the request path so rows never lag the response.
- [Risk] `sagaProcedure` test seam in service still returns the saga error only; hook behavior needs coverage → Mitigation: tasks add a fake `AuditStore` asserting one row per executed step plus the write-failure-spares-payment case.
