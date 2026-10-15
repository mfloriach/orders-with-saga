# Tasks

## 1. Broker infrastructure and config

- [x] 1.1 Add RabbitMQ service to `docker-compose.yml` (management-alpine image, ports 5672/15672, healthcheck, volume) and verify `docker compose up rabbitmq` reports healthy
- [x] 1.2 Add `RABBITMQ_URL` (required), `RABBITMQ_MAX_ATTEMPTS`, and `RABBITMQ_RETRY_TTL_MS` to `internal/config/config.go` with validation plus `.env.example`, and verify `go test ./internal/config/ -v` passes including a missing-URL boot failure case
- [x] 1.3 Add the AMQP client dependency and verify `go build ./...` passes

## 2. Broker wiring in app container

- [x] 2.1 Implement `internal/app/rabbitmq.go` `OpenBroker` (dial, declare `payments.events` exchange, `webhook.deliver` queue with DLX, `webhook.dlq`, bindings; fail-closed with cleanup) and verify with a live-broker test asserting topology exists
- [x] 2.2 Wire broker into `internal/app/container.go` `Build`/`Shutdown` and add a `rabbitmq` entry to `readyChecks` in `health.go`, and verify `go test ./internal/app/ -v` passes plus `GET /ready` reports the new dep

## 3. Webhook event publication

- [x] 3.1 Add publisher seam and AMQP implementation (event `{payment_id, merchant_id, amount, status}`, persistent delivery) with unit tests over a fake publisher, and verify `go test` on the new package passes
- [x] 3.2 Publish one event after saga success in `payment.Service.Create` (nothing on failure/in-doubt) and verify with a service-level test asserting publish-on-201 and no-publish-on-error

## 4. Worker binary

- [x] 4.1 Scaffold `cmd/worker/main.go` (load config, open broker + app DB, signal-aware shutdown draining in-flight deliveries) and verify the binary boots against compose and exits clean on SIGTERM
- [x] 4.2 Implement the consumer (resolve merchant, skip-with-ack when no `webhook_url`, POST JSON payload with timeout; 2xx acks, transient failures requeue with backoff up to the cap, then DLQ + identity log) and verify with live-broker tests covering deliver, skip, retry, and DLQ parking
- [x] 4.3 Add worker target to `Dockerfile` and verify `docker build` produces both binaries

## 5. Integration verification

- [x] 5.1 Run end-to-end against compose (create payment → 201 → merchant endpoint receives POST; kill endpoint → retries → DLQ entry + log) and verify each observable step
- [x] 5.2 Run `go test ./...`, `go vet ./...`, and `openspec validate --specs --change "rabbitmq-webhook-async"` and verify all pass
