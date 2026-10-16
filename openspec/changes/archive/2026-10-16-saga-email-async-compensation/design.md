# Design

## Context

See proposal.md (Why). Current state: `payment.Service.Create` runs a 4-step saga (insert, authorize, capture, webhook-publish) with synchronous unwind inside `saga.Run`; `steps.Email` is an unwired stub misnamed `"send-webhook"`; `steps/audit.go` (`StepInsertAudit`) is dead code not referenced by the `aa` map; `ledger_entries` is one-row-per-payment with no step/reason columns (and its `MarkFailed` references a `status` column the migration never created).

## Goals / Non-Goals

**Goals:** customer email without touching payment results; fast failure responses; a queryable per-step trail; saga contract change confined to unwind timing.

**Non-Goals:** saga retries, status vocabulary changes, worker-side audit rows, dedup.

## Decisions

- **Email outside the saga.** After `Run` returns nil, `Create` publishes the webhook event (existing) and the email event, then returns `201`. Email publish failure fails loudly like webhook publish does today (accepted residual risk, same posture). Rationale: a captured payment must never unwind because a notification failed.
- **Background unwind in `pkg/saga`.** On ordinary step failure: `Run` builds the `RunError` (without `CompensateErrs`), spawns unwind on a detached context using the existing `CompensateTimeout` logic extended to all unwinds, and returns immediately. Compensation errors become `Warn` logs with `saga_id`, `saga_name`, step. Unwind spans are re-rooted under a fresh background span since the request span may close on return. Alternative (bounded synchronous unwind) rejected per user decision.
- **Fail-marking stays synchronous.** The `MarkFailed` compensation path and the service-level failed marking on saga error stay synchronous in `Create` before returning. Background unwind only replays per-step compensations.
- **New `payment_step_events` table** `(id, payment_id, saga_id, step, outcome, reason NULL, created_at)` with no per-step uniqueness (replays append). Recording via a saga-level observer hook (e.g. `OnStepDone`), not per-step edits, so future steps are covered automatically. `AuditStore` gains `RecordStep`; dead `StepInsertAudit` is removed; `ledger_entries` untouched.
- **Fix `steps.Email` name to `"send-email"` first**, then implement as an event publisher on the webhook seam. The rename is load-bearing: the current duplicate name corrupts `unwind`'s `byName` map.
- **Audit write failures never fail payments.** A failing audit write logs loudly and continues; audit is observability, not money.

## Risks / Trade-offs

- [Crash mid-background-unwind] → Mitigation: the in-doubt/outbox reconciler must cover half-compensated payments; same change, not a follow-up.
- [`RunError.CompensateErrs` disappears as data] → Callers must not depend on it; logs become the only record. Accepted: speed over completeness.
- [Duplicate notifications on replay] → No saga retries (decided); worker redelivery is already at-least-once by design.
- [Audit table growth] → One row per step execution is bounded and indexed by `payment_id`; no retention policy in this change.

## Migration Plan

1. Land the `payment_step_events` migration (additive, no backfill).
2. Deploy saga + service + worker together (contract change is internal; no API change).
3. Rollback: revert binaries; the new table is ignored by old code.

## Open Questions

- None blocking. Email queue topology (new queue vs shared exchange binding) is an implementation detail within the existing broker.
