# Proposal

## Why

Payment creation is fully synchronous and its last saga step (`send-webhook`) is a no-op: merchants never receive delivery notifications, and a future synchronous webhook call would block the `201` response on an unreliable third-party URL. A durable async path with dead-letter handling unblocks real webhook delivery without slowing payment responses.

## What Changes

- Add RabbitMQ service to `docker-compose.yml` (broker + management UI ports, healthcheck, volume).
- Require `RABBITMQ_URL` at boot (same posture as `REDIS_ADDR`); extend fail-fast dependency check and shutdown to the AMQP connection.
- Publish a webhook event after saga success in `payment.Service.Create` via a small `Publish` seam (no AMQP imports in the service).
- New `cmd/worker` binary consuming `webhook.deliver`, POSTing to the merchant's `webhook_url`, with TTL-backoff retries and a dead-letter queue (`webhook.dlq`) for poison messages.
- Declare exchange/queues at boot; reconciliation outbox relay stays a later slice (DB remains source of truth).

## Capabilities

### New Capabilities

- `webhook-delivery`: async merchant webhook delivery — event publication on payment success, at-least-once delivery to `webhook_url`, bounded retries, and DLQ parking with log/alert.

### Modified Capabilities

- `service-config`: `RABBITMQ_URL` becomes required connection config; fail-fast boot check and shutdown cover the AMQP connection alongside Postgres/Redis.

## Impact

- New dependency: `rabbitmq:4-management-alpine` in compose; new Go AMQP client dependency.
- New binary `cmd/worker` (second Dockerfile target); `cmd/api` gains AMQP connection lifecycle.
- No API changes (still `201` on success); no migration changes; reconciliation outbox untouched.
- Out of scope: outbox-to-Rabbit relay, Redis consume-dedup, quorum queues, worker autoscaling.
