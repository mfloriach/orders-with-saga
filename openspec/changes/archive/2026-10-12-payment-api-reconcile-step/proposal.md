# Proposal

## Why

Payment `Create` currently compensates every failure backward (mark failed). The new last step calls an external API that cannot be undone: on ambiguous outcomes (timeout, 5xx, network error) blindly failing would strand a payment the remote may have accepted, and blindly succeeding could report success for a payment the remote rejected. The step needs retries plus synchronous reconciliation (status query), and a durable in-doubt path when confirmation is impossible.

## What Changes

- Add a last `call-api` saga step to `internal/payment.Service.Create` calling the external API with the request's idempotency key.
- Retry policy inside the step: retryable = timeouts, network errors, 5xx; never 4xx/business refusals. Bounded attempts + backoff + overall timeout (package constants; exact values land in review).
- Sync reconcile on still-ambiguous outcome: query remote status by idempotency key → confirmed success returns nil, confirmed absent/failure returns error (normal backward compensation of steps 1–2 follows).
- In-doubt signal in `pkg/saga`: a typed error from `Do` that makes `Run` skip unwind entirely and return it (keeps payment `pending` and the ledger entry intact). ~10 lines + test.
- In-doubt handling in service: insert a row into a new `reconciliation_outbox` table in the app DB (payment_id, idempotency_key, step, error, attempts, created_at) for next-day batch processing; payment stays `pending`; API returns `500` via the existing `InternalError` path.
- New `Processor` interface in the payment package (`Call` + `Status`, both keyed by idempotency key) injected into `Service`. No vendor client in this change beyond the seam; the concrete mapping lands at apply time.
- Out of scope: the next-day batch worker (reads the outbox later), parallel steps, saga retries.

## Capabilities

### New Capabilities

None. The outbox is storage supporting payment-processing behavior, not a standalone capability.

### Modified Capabilities

- `payment-processing`: new in-doubt requirement (stays `pending` + outbox row + `500`); existing pending/failed/500 semantics unchanged.
- `saga-library`: new in-doubt skip-unwind requirement; existing sequential/compensate/observability behavior unchanged.

## Impact

- New: `migrations/app/003_reconciliation_outbox.sql`, `Processor` seam + outbox repository method, saga in-doubt signal + tests.
- Modified: `internal/payment/service.go` (3rd step + in-doubt branch), `pkg/saga/saga.go`.
- No handler, config, or API-contract changes (in-doubt reuses `500`; client retry hits idempotency lookup → `409` with the pending payment).
- Assumption: migration file follows the existing `migrations/app/` convention and is applied by the operator's current process (no runner found in repo). Assumption: retry/confirm against the real vendor maps onto `Processor{Call, Status}` at apply time.
