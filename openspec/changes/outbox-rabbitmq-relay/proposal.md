# Proposal

## Why

Payment creation delivered merchant webhook and email notifications inline inside the saga after capture; when those calls failed the customer still received a 500 for a payment that had actually captured. The async half of this was built twice — first as a generic `outbox` event table (change `2026-10-21-add-payment-outbox`, since reverted), then re-landed on 2026-10-24 as the payment-centric `outbox_payments` table (status `pending` / `processed` / `failed`, attempts, last error) written by the saga capture step in the same transaction as the `captured` status change.

The relay worker that reads pending outbox rows and hands them to the message broker was never built, and the broker/consumer code was unwound during later refactors. Captured payments therefore have no durable path to asynchronous delivery: pending rows accumulate unread and nothing ever marks them processed.

## What Changes

- Add a relay worker that polls pending `outbox_payments` rows (claiming with `FOR UPDATE SKIP LOCKED`) and publishes a generic captured-payment event to the broker.
- On successful publish, mark the outbox row `processed` (write `processed_at`).
- On publish failure, increment `attempts` and record `last_error`, keeping the row `pending` for retry on the next poll; when attempts reach the bound, mark the row `failed`, alert, and leave it for operator replay.

## Capabilities

### New Capabilities

- `payment-outbox`: Durable captured-payment outbox with relay semantics — a captured payment is recorded as a pending outbox row, a relay worker hands it to asynchronous processing via the message broker, and the row is marked processed or failed as the relay lifecycle dictates.

### Modified Capabilities

- None in this change. The async delivery requirements in `webhook-delivery` and `email-notification` already exist; this change adds the missing relay handoff that future consumers bind to.

## Impact

- New: relay worker package + `cmd/worker` binary; broker wiring (`internal/config`, `internal/app`); `RABBITMQ_URL` and attempt configuration; generic captured-payment publish seam.
- Modified: outbox repository gains mark-processed / mark-failed / attempts methods; the saga capture step must pass the outbox repository into the capture step (currently not wired through container and registry).
- Existing: migration `migrations/app/007_outbox_payments.sql` already provides the table; no new migration.
- Broker: reuse the `payments.events` topic exchange with a generic captured-payment routing key.
- No API changes.
