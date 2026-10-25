# Design

## Context

See proposal.md - Why. Current state: the saga capture step writes an `outbox_payments` row (`pending`) in the same transaction as the `captured` status change, but the repository exposes only `Store` and `GetByStatus(pending)` — no mark-processed, mark-failed, or attempts methods. No worker reads the outbox, and the RabbitMQ wiring from the async-delivery effort was unwound in later refactors, so no broker client exists in the tree today. Durable specs (`webhook-delivery`, `email-notification`, `service-config`) already require async delivery with retries and DLQ parking, and a required `RABBITMQ_URL`.

## Goals / Non-Goals

**Goals:**

- Give captured payments a durable handoff path to asynchronous processing via the broker.
- Mark outbox rows processed on successful relay publish; failed relay publishes retry with bounded attempts and terminal failure handling.

**Non-Goals:**

- Consumer implementation (webhook/email delivery workers) — future change; this design only defines the relay contract.
- Broker-level dead-letter handling for relay publishes (retry is DB-driven via attempts, not message TTL/DLX).

## Decisions

**1. Outbox row is the relay's queue.** Pending rows are claimed with `FOR UPDATE SKIP LOCKED` and polled by the relay; status transitions drive lifecycle. Alternatives considered: broker-as-queue via direct saga publish, or a lease column — rejected in favor of the row-as-queue shape already settled in the `2026-10-16-queue-saga-unwind` design and re-confirmed by the user.

**2. Publish-then-mark ordering.** The relay publishes, then marks processed in a separate transaction. Mark-then-publish would lose the row on crash; publish-then-mark leaves a redelivery window that is absorbed by at-least-once delivery with idempotency keyed on payment id.

**3. Retry semantics.** On publish failure: increment `attempts`, write `last_error`, keep row `pending`. Retry happens on the next poll. At the attempt bound (3), mark the row `failed`, alert, and leave it for operator replay.

**4. Generic event payload.** The relay publishes one generic captured-payment event (routing on the existing `payments.events` topic exchange); consumer-specific payloads are a later concern.

## Risks / Trade-offs

- [Crash between publish and mark] → redelivery → dedup by payment id via consumer idempotency (accepted at-least-once).
- [Failed rows parked in DB] → alert + ops replay; no auto-requeue (accepted).

## Migration Plan

1. Add broker wiring, config, and worker binary.
2. Add outbox mark/attempts methods; wire outbox repository into the capture step.
3. Run relay against compose; verify end to end.

## Open Questions

None blocking.
