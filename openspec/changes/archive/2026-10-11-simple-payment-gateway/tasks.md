# Tasks

## 1. Scaffold + infra

- [x] 1.1 Scaffold Go module, `cmd/api/main.go`, Fiber app, `GET /health`, and verify `go build ./...` succeeds
- [x] 1.2 Add `docker-compose.yml` (app-db:5432, ledger-db:5433, redis:6379) with healthchecks plus `.env.example`, and verify `docker compose up -d` reports all healthy
- [x] 1.3 Add shared `config`, error envelope, and `validator` helper with handler-level `400` mapping, and verify `go vet ./...` passes

## 2. Merchant slice

- [x] 2.1 Add `migrations/app/001_merchants.sql` (merchants with unique email) and verify it applies cleanly to app-db
- [x] 2.2 Implement merchant repository + service + handler (`POST /merchants`) with manual DI wiring, and verify `POST /merchants` returns `201` and duplicate email returns `409` via curl

## 3. Payment slice

- [x] 3.1 Add `migrations/app/002_payments.sql` (unique merchant_id+idempotency_key) and `migrations/ledger/001_entries.sql`, and verify both apply cleanly
- [x] 3.2 Implement payment repositories (app + ledger), service orchestration with ledger-failure compensate to `failed`, and handler, and verify `POST /payments` returns `201`, unknown merchant `404`, duplicate key `409` with no new row, invalid amount `400`
- [x] 3.3 Wire Redis idempotency fast-path (`SET NX`, 24h TTL) with DB unique as truth, and verify repeated `POST /payments` with same key returns `409` with original ID

## 4. Rate limiting + verification

- [x] 4.1 Add Fiber limiter with Redis storage (100 req/min/IP, `429` + Retry-After), and verify 101 rapid requests return `429`
- [x] 4.2 Run end-to-end smoke (register → pay → duplicate-key → bad-input → rate-limit) against compose stack, and verify all status codes match specs
