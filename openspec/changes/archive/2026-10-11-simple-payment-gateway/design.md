# Design

## Context

Greenfield service; `openspec/specs/` empty. See `proposal.md` for why. Constraints from user: Go 1.27 + Fiber, `Handlers -> Services -> Repositories`, manual DI, `database/sql` + `pgx/stdlib`, Redis for idempotency + rate-limit, Kafka ignored in v1.

## Goals / Non-Goals

**Goals:**
- Two endpoints (`POST /merchants`, `POST /payments`) with validated JSON and consistent error envelope.
- Separate `app` (merchants, payments) and `ledger` (ledger_entries) Postgres databases.
- Idempotent payment creation and per-IP rate limiting backed by one Redis.

**Non-Goals:**
- Kafka producer/consumer, multi-currency, auth/API keys, refunds, webhooks delivery, OpenAPI, migration framework.

## Decisions

- **Manual constructor DI (`internal/app/container.go`):** `NewXxxRepository(db/redis)` → `NewXxxService(repo...)` → `NewXxxHandler(service)` wired in one `Build()` func. Alternative (Wire/Fx) rejected: no codegen/runtime graph needed for 2 slices.
- **Two `*sql.DB` pools via `pgx/stdlib`:** `APP_DSN`, `LEDGER_DSN` envs; plain `database/sql` + handwritten SQL + `migrations/*.sql`. Alternative (sqlc/ORM) rejected: 4 tables do not justify codegen.
- **Layer responsibilities:** Handler = bind + `validator.Struct` + `400` mapping only; Service = merchant-exists check, idempotency orchestration, ledger compensate; Repository = SQL/Redis calls only, no business rules.
- **Idempotency:** Redis `SET NX payments:idem:{merchant_id}:{key}` TTL 24h as fast-path + `UNIQUE(merchant_id, idempotency_key)` in `app.payments` as truth. Redis hit returns cached payment ID; DB unique violation → `409`.
- **Rate limit:** Fiber limiter middleware with Redis storage, `100 req/min/IP`, `429 + Retry-After`. Per-IP (not per-merchant) so unknown merchants are also protected.
- **Cross-DB consistency:** no 2PC. Insert payment (`pending`) in app DB → insert ledger entry → on ledger failure `UPDATE payments SET status='failed'`. Read path never reports success without ledger row.
- **Infra:** `docker-compose.yml` with `app-db:5432`, `ledger-db:5433`, `redis:6379`, named volumes + healthchecks. Kafka service omitted in v1 (compose addition is one block when needed).

## Risks / Trade-offs

- [Compensate vs 2PC leaves transient `failed` payments] → acceptable for v1; `failed` status is explicit, retry uses new idempotency key.
- [Redis eviction could lose fast-path entry] → DB unique constraint remains source of truth; worst case is one extra DB lookup.
- [Two Postgres instances double local resource use] → required by spec (ledger isolation); documented ports avoid collision.
- [`webhook_url` stored but never called] → stored validated only; delivery is a later change.

## Migration Plan

- Fresh deploy: `docker compose up -d`, apply `migrations/app/*.sql` and `migrations/ledger/*.sql`, start `cmd/api`.
- Rollback: stateless API; `docker compose down -v` resets local data. No data migration needed (v1 tables additive only).
