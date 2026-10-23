---
name: go-dev
description: Sets up and operates the standard Go project development environment — air hot reload, Docker Compose (Postgres, Redis, RabbitMQ), goose database migrations, Makefile targets, git pre-commit/pre-push hooks, and GitHub CI checks. Use whenever a Go project needs its dev environment set up, run locally, debugged with live reload, migrated, dockerized, or its lint/test/CI workflow wired or verified.
---

# Go Dev

Standard Go project setup. Reference implementation: this repository's config
files — `.air.toml`, `Makefile`, `docker-compose.yml`, `Dockerfile`,
`.github/workflows/ci.yml`, `.githooks/`, `migrations/`.

## Local dev loop

### Hot reload (air)

- Config lives at `.air.toml` in the repo root; `root = "."` must not change — the
  app resolves `.env` and `api/openapi.yaml` from the working directory.
- Install once: `go install github.com/air-verse/air@latest`
- Run: `make dev` (equivalent to `air -c .air.toml`).
- Run from the repo root only, never a subdirectory.

### Docker Compose

- Services: `app`, `app-db` (Postgres 16), `ledger-db` (Postgres 16), `redis`
  (7), `rabbitmq` (4, with management UI).
- Start: `make docker-up` (`docker compose --profile app up -d --wait`).
- Stop: `make docker-down`; wipe volumes: `make docker-clean`.
- Requires `.env` (copy from `.env.example`).

### Migrations (goose)

- Two databases: application (`migrations/app`) and ledger (`migrations/ledger`).
- Targets: `make migrate-up`, `make migrate-down`, `make migrate-status`.
- DSNs come from `.env` (`DB_DSN`, `LEDGER_DSN`); override per run:
  `make migrate-up DB_DSN=...`.
- New migration: `goose -dir migrations/app create <name> sql` (repeat for ledger).

## Quality gates

- `make lint` — gofmt check + `go vet ./...`
- `make build` — compile check `go build ./...`
- `make test` — `go test ./...`
- `make audit` — `govulncheck ./...` (installs the tool if missing)

## Git hooks

- `make hooks` — once per clone; sets `core.hooksPath` to `.githooks`.
- `pre-commit`: gofmt staged Go files and re-stage.
- `commit-msg`: conventional commits `<type>(<scope>): <subject>` with types
  `build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test`; subject ≤ 100 chars.
- `pre-push`: gitleaks scan + `make lint build test`.

## CI (GitHub)

`.github/workflows/ci.yml` runs on PRs and pushes to `main`, in this order:

1. `lint` → `make lint`
2. build → `make build`
3. test → `make test`
4. audit → `make audit` (govulncheck)
5. docker → buildx build + trivy scan (CRITICAL/HIGH, fail on findings) →
   `make docker-up` → `make docker-check` → dump logs on failure → `make docker-clean`

## Working agreement

- Before committing: `make hooks` (once), then `make lint build test` must pass.
- Before running the app: `make migrate-up`, then `make dev` or `make docker-up`.
- Before shipping a change that touches infra: run the CI pipeline locally via
  the same `make` targets — CI runs the identical commands.
