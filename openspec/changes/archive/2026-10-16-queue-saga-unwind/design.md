# Design

## Context

See proposal.md (Why). Current state: `saga.Saga.Run` compensates in-process via the `go` statement in `unwindAsync` (`pkg/saga/saga.go:154`), bounded by a 5s detached-context timeout; compensation errors are caller-process logs. `payment.Service.Create` builds steps from the `aa` registry (`PendingPayment`, `Authorize`, `Capture`, `Webhook`) whose `Compensate` closures capture the payment repo, card network client, and payment pointer. The broker topology (`payments.events` topic exchange + DLX, per-kind queues) and the consumer retry pattern (`x-attempts` header, republish-with-backoff, Nack-to-DLX) already exist in `internal/webhooks` + `internal/app/rabbitmq.go`; the worker runs one consumer per queue. The in-doubt path already writes a `reconciliation_outbox` row the same failure path can mirror.

## Goals / Non-Goals

**Goals:**
- No unwind is lost to process restarts; every owed unwind is recorded before the failure returns.
- The worker reuses the existing exchange/DLX/retry pattern; no new broker concepts.
- At-least-once execution without double side effects.

**Non-Goals:**
- Changing API semantics (still sync 500, same envelope) or adding a payment status.
- Generalizing `pkg/saga` into a queue-aware library; the queue seam lives in `payment`, not `saga`.
- A sweeper daemon for parked intents in this change (the row persists; replay is manual requeue).

## Decisions

**1. Outbox row is the queue (no dual write).**
`service.Create`, on saga failure, writes a `compensation_outbox` row (payment id, saga id, ordered completed steps, failed step, step error) in the same transaction scope as existing failure bookkeeping, and a relay publishes the message. Alternative (publish-then-return, memo row) rejected: commit-then-publish-fails leaves intent with no message and no publisher retry; row-as-queue makes publish failure retriable by the relay/sweeper instead of fatal.
- Alternative considered: transaction-outbox polling relay vs direct publish after commit. Direct publish after commit keeps this change small; the row remains the source of truth for orphans. A polling relay is a later optimization, not a correctness need.

**2. Message carries names, not code.**
Body: `{saga_id, saga_name, payment {id, merchant_id, amount, idempotency_key}, completed: [names...], failed_step, step_error}`. Closures cannot cross the wire, so the worker rehydrates steps from its own copy of the `aa` registry (same constructors as the service) and runs the recorded names in reverse. New topology: `CompensateRoute = "saga.compensate"`, `CompensateQueue = "saga.compensate"`, `CompensateDLQueue = "saga.compensate.dlq"` on the existing exchange/DLX.
- Alternative considered: one message per step (finer retry granularity). Rejected: reverse-order across messages needs a sequencer; a single ordered list keeps ordering trivial and matches one-saga-one-intent.

**3. `pkg/saga` stays broker-free.**
`saga.Run` stops spawning the unwind goroutine and instead returns the unwind plan (completed names + failure) in `RunError`; `payment.Service` translates that into the outbox row + publish. The saga package gains no AMQP/RabbitMQ import. Alternative (saga accepts a `QueueUnwind func(intent)` callback) deferred: return-the-plan keeps the seam to one new field and zero interface churn; a callback is equivalent power with more surface.

**4. Worker owns terminal marking via existing step compensations.**
The compensation consumer runs each recorded step's existing `Compensate` closure in reverse (insert-payment's `MarkFailed` still produces the terminal state), then acks. No separate "mark terminal" step: reuses tested logic, one code path for state changes. Consequence: every `Compensate` must become replay-idempotent (see Risks).

**5. Retry/DLQ mirrors the webhook consumer exactly.**
Same `x-attempts` header counting, same backoff, same Nack-to-DLX at cap, same log fields plus saga id. No new retry semantics to learn or operate.

## Risks / Trade-offs

- [Risk] Redelivered intent re-runs a non-idempotent compensation (e.g. `MarkFailed` on an already-`failed` row errors under the `WHERE status` guard; `CardNetwork.Capture` re-POSTs) → Mitigation: harden each `Compensate` to treat already-applied state as success (guard-clause on current row/network status first), covered by a replay test matrix in tasks.
- [Risk] Payment row misleads readers while unwind is queued (`authorized` that is doomed) → Mitigation: accepted by scope decision (side-table only); intent row is joinable by operators, and no polling readers exist today. Revisit if a GET payment endpoint lands.
- [Risk] Poison intent blocks the queue behind it → Mitigation: single-attempt-per-delivery (no inner retries, same as webhook consumer) plus DLQ cap keeps the queue moving; DLQ + logs page the operator.
- [Risk] Worker and API disagree on step registry (`aa` map drift) → Mitigation: single shared constructor for the registry used by both service and worker; unknown step name in an intent Nacks to DLQ loudly instead of skipping.
- [Risk] Publish-after-commit still has a crash window (row written, process dies before publish) → Mitigation: row is source of truth; startup/sweeper re-publishes unclaimed rows older than a bound (claims via `claimed_at`).

## Migration Plan

1. Deploy migration `004_compensation_outbox.sql` (additive table; no backfill).
2. Deploy API + worker together: API writes intents and publishes; worker consumes. Order matters — worker first is safe (empty queue), API first risks short queue buildup, both harmless.
3. Rollback: revert binaries; intents already consumed stay consumed (terminal marks are the same ones the old goroutine wrote). Unconsumed rows replay on re-deploy.

## Open Questions

None blocking. Replay bound for unclaimed rows (sweeper interval) is picked in tasks as a constant following the existing retry-TTL config pattern.
