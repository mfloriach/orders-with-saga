# Proposal

## Why

Payment creation still notifies nobody on the customer side, failures unwind synchronously inside the request (slow 500s), and there is no per-step record of what happened — only a terminal payment status. The `steps.Email` stub exists but is unwired and copy-pasted under the `"send-webhook"` name, which would corrupt saga unwind bookkeeping if wired as-is.

## What Changes

- Wire a real email step: publish an email event **after** the saga returns success (outside the saga, mirroring webhook delivery); fix the `steps.Email` step name to `"send-email"`.
- Failure path: mark the payment `failed` synchronously, return the error fast, and run saga compensation in the background with `saga_id`-tagged logs.
- Record one audit row per saga step execution (step, outcome, reason on failure) in a new `payment_step_events` table; leave `ledger_entries` as the money record.
- Keep `pending → authorized → captured` vocabulary unchanged; no retry logic in `pkg/saga` (transport retries stay in `apiclient`, delivery retries in the worker).

## Capabilities

### New Capabilities

- `payment-audit-trail`: per-step execution records (step name, outcome, failure reason, timestamp) for every payment creation attempt.
- `email-notification`: async customer email event published after successful payment creation, delivered by a background worker independently of the payment response.

### Modified Capabilities

- `payment-processing`: creation flow gains post-success email; failure marks `failed` synchronously while compensation completes in the background.
- `saga-library`: `Run` returns after marking failure without waiting for compensation; compensation errors are reported via logs (with `saga_id`), not in the returned error.

## Impact

- New migration (`payment_step_events`); `AuditStore` interface gains step recording.
- `pkg/saga` unwind becomes asynchronous (contract change for all current and future callers).
- New email publish path in `payment.Service` + worker delivery.
- No API changes (still `201`/`409`/`500` as today); no status vocabulary changes.
- Out of scope: saga-level retries, consume-side dedup, worker audit rows (saga path only).
