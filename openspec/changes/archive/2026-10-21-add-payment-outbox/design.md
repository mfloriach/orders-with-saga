# Design

## Context

Payment creation runs a saga of four steps (authorize → capture → webhook → email) synchronously inside the HTTP request. Steps after capture call external services (merchant webhook, SMTP). The `payments` table only allows `pending`/`authorized` → `failed` transitions, so when webhook or email fails after capture, `Payments.Failed()` matches zero rows on a `captured` payment and returns an error — the customer gets a 500 for a payment the card network already captured.

The project already uses PostgreSQL with outbox-adjacent tables (`reconciliation_outbox`, `compensation_outbox`) though neither is referenced in Go code. Migrations live under `migrations/app/` and are numbered sequentially (latest is `006`).

## Goals / Non-Goals

**Goals:**
- Webhook and email delivery no longer block the payment creation response.
- A captured payment is never marked failed because a notification could not be delivered.
- Every notification side-effect is durably recorded in an outbox row written in the same transaction as the payment state change.
- Processed outbox rows are deleted after successful processing.

**Non-Goals:**
- The outbox consumption worker (delivery, retries, dead-letter queue) — explicitly deferred per user decision.
- Changes to `authorize` or `capture` — they remain synchronous money-state mutations.
- Refund flow outbox events.

## Decisions

**New outbox table.** Add a migration (`007_outbox.sql`) creating an `outbox` table with columns: `id` UUID PK, `aggregate_type` TEXT, `aggregate_id` UUID, `event_type` TEXT, `payload` JSONB, `created_at` TIMESTAMPTZ, `processed_at` TIMESTAMPTZ nullable. Rows are soft-deleted physically (DELETE) after processing. Rationale: keeps the table small, matches the project's existing outbox tables which use simple schemas, and avoids adding a `status` column the worker isn't ready to consume yet.

**Same-transaction write.** Each saga step that produces a side-effect (webhook, email) writes its outbox row in the same database transaction as the payment state transition. Rationale: atomicity — either the payment and the event row both commit, or neither does. The outbox row is the source of truth for eventual delivery.

**Saga step returns success after outbox write.** `webhook.Do()` and `email.Do()` write an outbox row and return `nil`. Payment creation returns `201` with `captured` status regardless of whether the notification was delivered. Rationale: payment success must not depend on external service availability.

**No new repository type yet.** Reuse `PaymentDatabaseRepository` for outbox writes (add an `OutboxEvent` method) rather than introducing a new repository, keeping the change surface small. A dedicated outbox repository can be added when the worker is built.

## Risks / Trade-offs

[Orphaned outbox rows] → If processing never runs (worker deferred), rows accumulate. Mitigation: rows are small JSONB payloads; a future sweeper can backfill. Documented as accepted.

[Duplicate delivery] → Without a worker, no de-duplication logic exists yet. Mitigation: the worker (deferred) will own idempotent delivery using event id; outbox rows carry a unique id.

[Payment marked captured but notification never sent] → Accepted per the spec: email/webhook are best-effort after capture; the payment itself is durable and correct.
