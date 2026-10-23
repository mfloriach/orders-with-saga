# Payment Gateway

A payment gateway API written in Go. It manages merchants and processes payments
through a saga of steps: authorize with the card network, capture, notify the
merchant by webhook, and email a receipt. Every step is compensable, and failed
payments are recorded before the error is returned.

- HTTP: [Fiber v2](https://gofiber.io)
- Database: PostgreSQL (two pools: application + ledger)
- Cache / coordination: Redis
- Observability: structured JSON logs with `request_id` and `trace_id`, OpenTelemetry spans (no exporter configured yet)
- API reference: served at `/docs`, machine-readable spec at `/openapi.yaml` (`api/openapi.yaml`)

## Requirements

- Go 1.27+
- Docker (for Postgres and Redis)

## Quick start

```sh
cp .env.example .env      # defaults match docker-compose.yml
docker compose up -d      # Postgres (app + ledger) and Redis

go install github.com/pressly/goose/v3/cmd/goose@latest   # once
make migrate-up           # applies migrations/app, then migrations/ledger
go run ./cmd/api
```

Then:

- API reference UI: <http://localhost:8080/docs>
- Liveness: `curl localhost:8080/health` → `{"ok":true}`
- Readiness (checks both databases and Redis): `curl localhost:8080/ready`

Or run everything, API included, in Docker:

```sh
make docker-build docker-up
curl localhost:8080/ready
make docker-down          # stop the stack (keeps volumes)
make docker-clean         # stop the stack and delete its volumes
```

Payment creation calls the card network. Without `MC_BASE_URL` set, that call
fails and payment creation fails closed — merchant endpoints still work.

`docker compose` reads the same `.env`; service credentials and host ports are
overridable with `APP_DB_*`, `LEDGER_DB_*`, `REDIS_PORT`, and `RABBITMQ_*`
(defaults match the DSNs).

### Creating a payment

```sh
curl -s localhost:8080/payments \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: order-123' \
  -d '{"merchant_id":"<uuid>","price":{"amount":2500,"coin":"USD"}}'
```

Flow: validate → merchant must exist → idempotency check → insert `pending` →
saga (`authorize` → `capture` → `webhook` → `email`) → return `201` with the
captured payment. Any failed step runs the compensations of the steps that
already succeeded and marks the payment `failed`.

### Idempotency

- The optional `Idempotency-Key` header is deduplicated per merchant over the
  payments table (`UNIQUE (merchant_id, idempotency_key)`). Reusing a key
  returns `409` naming the original payment.
- Without a header, a fresh key is derived per request (`drv:` + UUID), so
  keyless requests are never deduplicated against each other.
- The key actually used is echoed in the `Idempotency-Key` response header.
- Amounts are integer minor units; `coin` is an ISO 4217 code (currently
  `USD`, `EUR`, `JPY`).

## Development

```sh
make hooks   # once per clone: enable the git hooks in .githooks/
make lint    # gofmt check + go vet
make build
make test
```

The hooks format staged Go files, enforce conventional commit messages, and on
push run [gitleaks](https://github.com/gitleaks/gitleaks) over the pushed
commits (install it with `brew install gitleaks`) before `make lint build test`.

Live reload (rebuilds and restarts on Go/template/`.env` changes):

```sh
go install github.com/air-verse/air@latest
make dev     # equivalent to: air -c .air.toml
```

`air` runs the binary from the repository root, which is what lets the app load
`.env` and `api/openapi.yaml`. Do not run it from a subdirectory.

### Migrations

Migrations use [goose](https://github.com/pressly/goose) and live in
`migrations/app` (application DB) and `migrations/ledger` (ledger DB). Each file
has `-- +goose Up` and `-- +goose Down` sections.

```sh
make migrate-up       # apply pending migrations to both databases
make migrate-down     # roll back one migration in each database
make migrate-status   # show applied and pending migrations
```

The targets read `DB_DSN` / `LEDGER_DSN` from `.env`; override on the command
line when needed (`make migrate-up DB_DSN=...`). Create a new migration with:

```sh
goose -dir migrations/app create add_refunds sql
```

If `go install` fails while linking cgo, retry with `CGO_ENABLED=0`:
`CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@latest`.

## Layout

```
cmd/api                 entrypoint: config, container wiring, graceful shutdown
internal/app            Fiber server, routes, health/readiness, API docs
internal/config         env loading, Postgres and Redis pools
internal/domain         core types: merchant, payment, lifecycle statuses
internal/merchant       merchant HTTP handlers, service, repository
internal/payment        payment handler, service, saga step definitions
internal/payment/...    Postgres repositories, card-network and webhook steps
internal/notifications  SMTP sender and email templates
pkg/apiclient           HTTP client with timeout and retry/backoff
pkg/apperr              API error model and Fiber error handler
pkg/idempotency         key derivation
pkg/logging             slog setup, request IDs, trace correlation
pkg/saga                generic compensable saga executor
pkg/tracing             OpenTelemetry tracer provider
pkg/validate            request validation helpers
api/openapi.yaml        OpenAPI contract
migrations/             goose migrations for the app and ledger databases
.air.toml               live-reload configuration (air)
```
