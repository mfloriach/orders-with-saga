# Design

## Context

See proposal.md (Why). Current state: `internal/app/container.go:61-99` opens Postgres x2 + Redis in `Build`, failing closed per dependency; `internal/app/redis.go` is the pattern for open-with-ping; `internal/app/health.go:43-49` wires `readyChecks` over the same pools. `payment.Service.Create` ends with the `send-webhook` saga step (`internal/payment/steps/webhook.go`), currently a no-op returning nil. Merchants carry an optional `webhook_url` (`internal/merchant/model.go`). `cmd/api/main.go` runs one Fiber server with SIGINT/SIGTERM shutdown via `Container.Shutdown`.

## Goals / Non-Goals

**Goals:**
- Webhook delivery leaves the request path: `Create` returns `201` without calling any merchant URL.
- Every published event is delivered at least once or parked in the DLQ with identity logs.
- `cmd/worker` crashes and restarts lose no acknowledged work (manual acks, persistent messages).

**Non-Goals:**
- Reconciliation-outbox relay (later slice; table stays the source of truth).
- Consume-side dedup (deferred); ordering guarantees across payments.
- Quorum queues / mirrored-cluster topology; single-node RabbitMQ with durable classic queues.

## Decisions

- **Topology: one topic exchange, two queues + DLX.** Exchange `payments.events`; queue `webhook.deliver` (durable, `x-dead-letter-exchange: payments.dlx`) bound by `webhook.*`; DLQ `webhook.dlq` bound on `payments.dlx`. Retry republishes to `webhook.deliver` with an `x-attempts` header after a backoff sleep (TTL x attempt), capped (default 5); unparseable and over-cap messages are Nack'd with requeue=false so the DLX parks them. Alternative (dedicated TTL wait queues, per-retry delay queues) rejected: header-counted republish needs no extra queues for one consumer.
- **Publisher seam, not AMQP in the service.** New `pkg/webhookevents` (or `internal/payment/events`) interface `Publisher { Publish(ctx, event) error }` with an AMQP implementation; `payment.Service` takes it as a constructor arg and hands it to the `send-webhook` saga step. Mirrors the existing `CardNetwork` seam: service stays broker-agnostic and unit-testable. Event published as the last saga step, before returning `201`.
- **Declaration at boot, fail-closed.** `internal/app/rabbitmq.go` `OpenBroker(cfg)` dials, declares exchange/queues/bindings, and pings — same shape as `OpenRedis`. `Build` wires it; failure closes already-opened pools and aborts boot. Alternative (lazy declare on first publish) rejected: hides misconfiguration until the first payment.
- **Separate `cmd/worker` binary.** Own `main.go`: loads config (same required `RABBITMQ_URL`), opens broker + app DB (merchant lookup) + logger/tracer, consumes `webhook.deliver` with manual acks, exits on SIGINT/SIGTERM after in-flight deliveries finish. Alternative (in-process consumer in `cmd/api`) rejected per user decision: independent scaling and no coupling of API deploys to delivery backlog.
- **Delivery semantics.** Worker resolves merchant, skips-with-ack when no `webhook_url`, else POSTs JSON `{payment_id, merchant_id, amount, status}` with a short timeout; 2xx acks, anything else requeues-with-backoff or DLQs at the cap. Logs carry `payment_id`, attempt count, and failure cause.
- **Config style follows `config.go`.** `RabbitMQURL string env:"RABBITMQ_URL" env-required:"true"`, plus `RABBITMQ_MAX_ATTEMPTS` (default 5) and `RABBITMQ_RETRY_TTL_MS` validated like the `MC_*` fields. `readyChecks` gains a `rabbitmq` entry; `Shutdown` closes the AMQP connection first (stop intake before pools).

## Risks / Trade-offs

- [Publish-then-crash duplicate] → Worker may deliver twice after a crash between publish and response; merchant endpoints must be idempotent (documented; dedup deferred).
- [Broker down blocks payments] → Fail-closed boot plus publish errors fail `Create` loudly rather than silently dropping events; `/ready` exposes it. Accepted: silent loss is worse than a `500`.
- [New client dependency] → One AMQP library (e.g. `amqp091-go`); confined to `internal/app/rabbitmq.go`, the publisher implementation, and the worker.
- [Poison-message buildup] → DLQ is bounded by alerting on depth via the management UI/logs, not auto-replay; manual replay is out of scope.

## Migration Plan

1. Deploy compose with RabbitMQ alongside existing services; no migration files.
2. Deploy `cmd/api` (publishes events; worker not yet running is safe — queue buffers durably).
3. Deploy `cmd/worker`; verify delivery + DLQ parking in staging via management UI (`:15672`).
4. Rollback: stop worker, revert api; queued events drain on re-deploy. No data migration either direction.

## Open Questions

- None blocking. Attempt cap default (5) and retry TTL are tunable via env and can be adjusted after observing merchant endpoint behavior.
