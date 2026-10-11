# Proposal

## Why

No payment backend exists. Merchants cannot be registered and payments cannot be accepted, so we need a minimal gateway foundation (two endpoints + local infra) that later features (refunds, webhooks, Kafka events) can extend.

## What Changes

- Add `POST /merchants` to register a merchant (`name`, `email`, optional `webhook_url`).
- Add `POST /payments` to create a payment (`merchant_id`, `amount` in minor units, `idempotency_key`); no multi-currency in v1.
- Add `docker-compose.yml` with two Postgres databases (`app`, `ledger`) and Redis; Kafka explicitly out of scope for v1.
- Scaffold Go 1.27 + Fiber service with `Handlers -> Services -> Repositories` layering, manual constructor DI, request validation with `go-playground/validator`.
- Use `database/sql` with `pgx/stdlib` driver (one `*sql.DB` pool per Postgres); Redis for idempotency fast-path and rate limiting.

## Capabilities

### New Capabilities

- `merchant-registration`: register and look up merchants; owns merchant identity and validation rules.
- `payment-processing`: create payments with idempotency, ledger recording, and rate limiting; owns payment lifecycle in v1.

### Modified Capabilities

None — greenfield project, `openspec/specs/` is empty.

## Impact

- New Go service (`cmd/api`, `internal/`, `pkg/`), `Dockerfile`, `docker-compose.yml`, `.env.example`.
- New HTTP API surface: `POST /merchants`, `POST /payments` (+ `GET /health`).
- New runtime dependencies: Postgres x2, Redis. No Kafka, no auth, no external PSP calls in v1.
