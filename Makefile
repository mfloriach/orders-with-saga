# Local config for migrate targets (DSNs). Real env vars can still be passed
# on the make command line, e.g. `make migrate-up DB_DSN=...`.
-include .env
export

GOOSE ?= goose
APP_MIGRATIONS ?= migrations/app
LEDGER_MIGRATIONS ?= migrations/ledger
IMAGE ?= paymentgateway:local
APP_IMAGE ?= $(IMAGE)
DOCKER_CACHE ?=
COMPOSE ?= docker compose --profile app

.PHONY: hooks dev lint build test audit migrate-up migrate-down migrate-status \
	docker-build docker-up docker-check docker-logs docker-down docker-clean

hooks:
	git config core.hooksPath .githooks

dev:
	air

migrate-up:
	$(GOOSE) -dir $(LEDGER_MIGRATIONS) postgres "$(LEDGER_DSN)" up
	$(GOOSE) -dir $(APP_MIGRATIONS) postgres "$(DB_DSN)" up

migrate-down:
	$(GOOSE) -dir $(APP_MIGRATIONS) postgres "$(DB_DSN)" down
	$(GOOSE) -dir $(LEDGER_MIGRATIONS) postgres "$(LEDGER_DSN)" down

migrate-status:
	@echo "app:"
	@$(GOOSE) -dir $(APP_MIGRATIONS) postgres "$(DB_DSN)" status
	@echo "ledger:"
	@$(GOOSE) -dir $(LEDGER_MIGRATIONS) postgres "$(LEDGER_DSN)" status

lint:
	test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	go vet ./...

build:
	go build ./...

test:
	go test ./...

audit:
	@command -v govulncheck >/dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

docker-build:
	docker buildx build $(DOCKER_CACHE) --load -t $(IMAGE) .

docker-up:
	$(COMPOSE) up -d --no-build --wait --wait-timeout 120

docker-check:
	curl -fsS http://localhost:$${PORT:-8080}/health | grep -q '"ok":true'
	curl -fsS http://localhost:$${PORT:-8080}/ready | grep -q '"ok":true'
	$(COMPOSE) logs --no-color app | grep -q '"msg":"app started"'

docker-logs:
	$(COMPOSE) logs --no-color

docker-down:
	$(COMPOSE) down

docker-clean:
	$(COMPOSE) down -v
