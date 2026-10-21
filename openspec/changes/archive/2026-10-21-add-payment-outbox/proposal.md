# Proposal

## Why

Payment creation blocks the HTTP response on webhook and email delivery after the card network has already captured funds. If either external call fails, the payment is already `captured` but `Payments.Failed()` cannot transition `captured → failed`, so the customer receives a 500 for a payment that actually succeeded. The outbox pattern makes notification delivery durable and non-blocking so payment success is never held hostage by an unreliable merchant endpoint or mail server.

## What Changes

- Add a new `payment-outbox` capability with an `outbox` table: one event row per payment-side-effect (webhook, email), written in the same transaction as the payment state change.
- Change `webhook` and `email` saga steps from synchronous, response-blocking delivery to asynchronous outbox publication — the step writes an event row and returns success; payment creation returns `201` with `captured` status regardless of delivery outcome.
- Delete processed outbox rows after successful delivery (per the reconciliation/cleanup model the project already uses for outbox tables).
- Ignore the delivery worker for now — this change only adds the outbox table and the step-level publishes; consumption is out of scope.

## Capabilities

### New Capabilities

- `payment-outbox`: Durable outbox table for payment-side-effect events. Covers the schema, the event-publish contract within saga steps, and delete-after-process semantics. The background worker that consumes rows is intentionally out of scope.

### Modified Capabilities

- `webhook-delivery`: Webhook events are now published to the outbox table instead of being delivered synchronously during payment creation. Async background delivery (already specified) remains the path to the merchant endpoint; the synchronous saga-step call becomes an outbox publish that cannot fail the payment response.
- `email-notification`: Email events are now published to the outbox table instead of being delivered synchronously during payment creation. Async background delivery (already specified) remains the path to the customer mailbox; the synchronous saga-step call becomes an outbox publish that cannot fail the payment response.

## Impact

- New migration creating the `outbox` table under `migrations/app/`.
- `internal/payment/workflow/webhook.go` — `Do()` writes an outbox event row instead of calling the merchant webhook synchronously.
- `internal/payment/workflow/email.go` — `Do()` writes an outbox event row instead of calling the SMTP sender synchronously.
- `internal/domain/interfaces.go` — `PaymentRepository` may need an outbox-write method, or a new outbox repository type.
- `openspec/specs/payment-processing/spec.md` — requirement deltas for webhook and email steps no longer blocking the response.
- No changes to `authorize` or `capture` — they remain synchronous money-state mutations.
