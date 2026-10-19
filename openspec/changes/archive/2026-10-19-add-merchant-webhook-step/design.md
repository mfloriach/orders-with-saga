# Design

## Context

Current state (see proposal.md for motivation):
- `internal/payment/workflow/` runs a saga v2 (`pkg/saga/v2`) with steps `authorize → capture`, wired in `registry.go` via `NewWorkflow(payments, cardNetwork)`.
- `pkg/apiclient` is a stdlib HTTP client with bounded retries, exponential backoff + jitter, per-attempt timeout, and a retry predicate. Already used by `MasterCardRepository` (card network) and `webhooks.consumer.webhookNotifier` (single-attempt merchant POST).
- `internal/webhooks/` defines `webhooks.Event` (`payment_id`, `merchant_id`, `amount`, `status`) and a `Publisher` interface; `WebhookPublisher` (AMQP) exists in `internal/app` but is not wired into payment creation.
- `internal/merchant/repository.go` satisfies `MerchantLookup` (`FindByID`) and exposes `WebhookURL`.
- Config has `MC_*` timeout/retry settings; no webhook-specific config exists.

Constraints:
- Webhook-delivery spec previously guaranteed merchant endpoints never block payment responses; this change removes that guarantee for the sync path.
- No new config env vars: reuse `MC_*` for the webhook client timeout/retry budget.
- Saga compensation runs completed steps in reverse order; the failed step's own `Compensate` does not run.

## Goals / Non-Goals

**Goals:**
- Add a synchronous `webhook` saga step after `capture` that POSTs a `webhooks.Event` to the merchant's `webhook_url` via `pkg/apiclient`.
- On webhook failure, mark payment `failed`, return `500`, and compensate `capture` then `authorize` in reverse order.
- `Compensate` re-runs `Do` (re-POST), mirroring the existing `authorize` compensation pattern.
- Reuse `webhooks.Event` as the payload; no new event type.

**Non-Goals:**
- Async webhook delivery via the broker (already exists; out of scope).
- New config env vars or config changes.
- Email notification changes.
- Merchant-side idempotency/deduplication of webhook calls (merchant's responsibility).
- Changes to `authorize` or `capture` steps.

## Decisions

| Decision | Rationale | Alternatives considered |
|---|---|---|
| Sync saga step (blocks payment response) | User selected Option A: merchant notification is part of the creation flow; failure unwinds the payment. | Async via existing broker publisher (Option B): keeps response unblocked but contradicts the user's blocking-notification requirement and adds compensation hazard if capture already succeeded. |
| `Compensate` re-runs `Do` (re-POST) | Mirrors the existing `authorize.Compensate` pattern; minimal new code, one consistent compensation style. | No-op compensate: simpler but leaves a sent notification un-reconciled. Compensating undo (e.g. reverse notification): not supported by merchant endpoints; over-engineered. |
| Reuse `MC_*` config for webhook client | No webhook config exists; adding env vars is scope creep. `MCTimeout`, `MCMaxAttempts`, `MCBaseBackoff`, `MCMaxBackoff` are appropriate for an outbound HTTP call. | New `WEBHOOK_*` env vars: more config to own, operate, and document. |
| Reuse `webhooks.Event` as payload | Already carries `payment_id`, `merchant_id`, `amount`, `status` — exactly what a merchant webhook needs. | New struct: duplication with no added value. |
| Add `merchantStore` (`FindByID`) to `NewWorkflow` | The webhook step needs the merchant's `webhook_url`; `Payments` and `CardNetwork` alone don't provide it. | Pass merchant URL into the workflow struct at execution: leaks merchant lookup into the caller; less cohesive. |

## Risks / Trade-offs

- [Webhook failure unwinds a captured payment] A failed merchant notification compensates `capture`, undoing a successfully captured payment because notification failed. → Mitigation: this is the chosen trade-off per Option A; document in the proposal that notification failure fails the payment.
- [Compensate re-POST may duplicate notifications] Re-running `Do` re-POSTs to the merchant; if the merchant endpoint isn't idempotent, the merchant receives duplicate webhooks. → Mitigation: include `payment_id` in the event so merchants can deduplicate; this matches the existing `authorize` compensation risk profile.
- [Sync call increases payment latency] The payment response now waits on an outbound HTTP call to the merchant. → Mitigation: bounded by `MCTimeout` and `MCMaxAttempts`; merchant endpoints should be fast.
- [No webhook-specific retry classification] `apiclient` retries transport errors and non-2xx by default; a merchant returning 500 will be retried, which may be undesirable. → Mitigation: pass a custom `retryIf` if needed; deferred as an open question.

## Open Questions

- Should the webhook client use a custom `retryIf` that only retries transport errors (not non-2xx), to avoid retrying a merchant's hard 4xx/500 refusal? Defer until the first production incident; can be added via `apiclient.WithRetry` without changing the step signature.
