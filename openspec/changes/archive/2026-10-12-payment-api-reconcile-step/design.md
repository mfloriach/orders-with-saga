# Design

## Context

See proposal.md — Why. Current state: `internal/payment/service.go:42-61` runs a 2-step saga (`insert-payment`, `insert-ledger`), both compensatable; any `Do` error triggers reverse unwind via `pkg/saga/saga.go:147-184`. No external client exists (v1 explicitly excluded PSP calls). `payments.status` is `CHECK (status IN ('pending','failed'))`, so in-doubt reuse of `pending` needs no constraint change. Migrations are plain SQL files under `migrations/app/` (`001`, `002`); no runner found in repo — operator applies them as before.

## Goals / Non-Goals

**Goals:**
- Last-step external call with bounded retries + sync status-query reconcile.
- Explicit in-doubt path: skip unwind, stay pending, durable outbox row.
- Saga lib learns one concept (in-doubt skip), nothing else changes.

**Non-Goals:**
- Batch worker, saga-level retries, parallel steps, concrete vendor SDK (seam only).

## Decisions

**1. In-doubt as typed skip-unwind signal in saga (option b from exploration).**
`Do` returns a recognized in-doubt error (e.g. `*saga.InDoubtError` carrying step + cause); `Run` detects it via `errors.As` before unwinding, skips `unwind` entirely, and returns a `RunError` flagged in-doubt with `Completed` intact. Rationale: the alternative (Do swallows in-doubt, smuggles outcome via closure, returns nil) logs saga "success" for a non-success and hides control flow. Cost is ~10 lines + 1 test. Rejected alternative (a) for that reason.

**2. Retries + reconcile inside the step's `Do`, not the saga.**
Classifier: retryable = timeouts, temporary network errors, 5xx / ambiguous; terminal = 4xx and business refusals. Flow: call → on ambiguous, retry with backoff up to N attempts (same idem key) → on success nil; on terminal or query-confirmed-absent, error; on exhausted + query-inconclusive, in-doubt error. Rationale: saga keeps its no-retry non-goal; retry policy is a payment-domain concern. Constants (`maxAttempts`, backoff, per-attempt + overall timeout) live in the payment package; exact values set in review.

**3. `Processor` seam injected into `Service`.**
Minimal interface: `Call(ctx, paymentID, amount, idemKey)` + `Status(ctx, idemKey) -> (accepted bool, err error)`, plus a `Retryable(err) bool` classifier owned by the implementation. Rationale: no vendor SDK in this change; the seam is unit-testable with stubs (matching existing `handler_test.go` stub style) and the concrete client maps onto it at apply time.

**4. Outbox table in app DB: `reconciliation_outbox(payment_id UUID PK REFERENCES payments(id), idempotency_key TEXT NOT NULL, step TEXT NOT NULL, error TEXT NOT NULL, attempts INT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`.**
Rationale: app DB owns payments; ledger DB stays append-only. Columns are the minimum a future batch needs (identify, retry with same key, count attempts). Insert happens in service after `Run` returns in-doubt, outside the saga. `003_reconciliation_outbox.sql` follows existing naming.

**5. In-doubt responds 500 via existing `InternalError`; no handler change.**
Rationale: no new API contract; client retry hits `FindByIdem` → existing 409 with the pending payment id, which the client polls on.

## Risks / Trade-offs

- [Sync reconcile holds the request] → bounded by attempt cap + overall timeout; exceeding budget IS the in-doubt trigger, never an unbounded wait.
- [Outbox insert fails after in-doubt] → payment still `pending`; log loudly; fallback is a future sweeper scanning stale pendings. Accepted: insert failure is rarer than API ambiguity.
- [Double-execution via retry] → mitigated only because every call/query carries the idempotency key; the concrete client MUST honor it (apply-time verification task).
- [Pre-existing drift, out of scope] → `LedgerRepository.MarkFailed` updates `ledger_entries.status`, but `migrations/ledger/001_entries.sql` defines no `status` column — the insert-ledger compensation errors at runtime today (surfaced as `CompensateErrs`). Flagged, not fixed here; the batch/in-doubt path does not depend on it.

## Migration Plan

1. Land saga in-doubt signal + tests (no callers; inert).
2. Land migration `003`, outbox repository, `Processor` seam + service 3rd step; existing payment handler tests cover regression.
3. Rollback = revert service commit (outbox table unused is inert); saga signal unused is inert.

## Open Questions

None that change specs, approach, or tasks. Retry constants and vendor mapping resolve in review/apply.
