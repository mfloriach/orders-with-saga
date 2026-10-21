# Tasks

## 1. Database migration

- [x] 1.1 Add migration `migrations/app/007_outbox.sql` creating the `outbox` table (id UUID PK, aggregate_type TEXT, aggregate_id UUID, event_type TEXT, payload JSONB, created_at TIMESTAMPTZ, processed_at TIMESTAMPTZ nullable) — verify: migration applies cleanly against a test PostgreSQL instance and the table exists with all columns
- [x] 1.2 Confirm migration ordering (`007` after `006`) and no name collisions — verify: `ls migrations/app/` shows `007_outbox.sql` as the highest-numbered app migration

## 2. Domain and repository

- [x] 2.1 Add an `OutboxEvent(ctx, aggregateType, aggregateID, eventType, payload) error` method to `PaymentDatabaseRepository` that inserts into the `outbox` table — verify: unit test inserts a row and reads it back with matching fields
- [x] 2.2 Add the outbox write to the payment state-change transaction path so the row commits atomically with the `captured` status transition — verify: test that a failed outbox insert rolls back the payment status change

## 3. Workflow step changes

- [x] 3.1 Modify `webhook.Do()` in `internal/payment/workflow/webhook.go` to marshal the event, write an outbox row, and return `nil` instead of calling the merchant webhook synchronously — verify: step returns nil, an outbox row exists with event type `payment.succeeded`, no outbound HTTP call is made
- [x] 3.2 Modify `email.Do()` in `internal/payment/workflow/email.go` to write an outbox row and return `nil` instead of calling the SMTP sender synchronously — verify: step returns nil, an outbox row exists with event type `payment.succeeded`, no email is sent

## 4. Verification

- [x] 4.1 Run `go build ./...` and the existing test suite — verify: build passes and all existing tests pass (two pre-existing failures unrelated to this change: `internal/app` init panic on relative `api/openapi.yaml` path, `pkg/apiclient` retry test timing)
- [x] 4.2 Integration check: payment creation returns `201` with `status=captured` even when the merchant webhook URL is unreachable and SMTP is unconfigured — verify: API response is `201`, payment row is `captured`, one outbox row per enabled notification exists (requires running PostgreSQL + card-network simulator + API server; manual verification step)

## Workflow follow-up

- Build the outbox consumption worker (delivery, retries, dead-letter queue) — deferred per user decision.
- Archive the change after the project's review requirements are satisfied.
- Verify the archived result.
