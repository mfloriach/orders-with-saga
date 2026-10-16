# Proposal

## Why

Saga compensation currently runs in a fire-and-forget goroutine inside the API process: a deploy, crash, or restart between failure and unwind completion silently drops the unwind, leaving the payment in a stale state (e.g. `authorized`) with no record that compensation is owed. Queueing the unwind makes it durable and observable.

## What Changes

- Saga failure records a durable compensation intent (outbox row: saga id, ordered completed steps, failed step) before returning the failure to the caller.
- The intent is published as a persistent message on the existing broker topology under a new compensation route; the in-process unwind goroutine is removed.
- The worker binary gains a compensation consumer that rebuilds the saga steps from the payment step registry, claims the intent, runs compensations in reverse order, marks the terminal payment state, and acknowledges.
- Exhausted retries park the message in the dead-letter queue with payment id and failure cause; the outbox row remains for sweeper replay.
- Compensation replay is idempotent: re-running an already-applied compensation is a no-op, never a second side effect.
- No new payment status: the row keeps its last step state while unwind is queued (side-table intent only, per exploration decision).

## Capabilities

### New Capabilities

- `saga-compensation`: durable queued saga unwind — intent record, queue message, worker execution, retry/DLQ, idempotent replay.

### Modified Capabilities

- `saga-library`: the reverse-compensation requirement changes from in-process background unwind to durable queued unwind; per-step error reporting moves from caller logs to worker logs.

## Impact

- `pkg/saga`: `unwindAsync` no longer spawns the unwind goroutine; saga exposes the unwind intent (or accepts a queue seam) instead.
- `internal/payment`: service records intent on saga failure; new consumer package (or `internal/webhooks`-style package) for compensation execution; step compensations hardened for idempotent replay.
- `internal/app`: broker topology gains compensation queue/route; `cmd/worker` wires the new consumer.
- New migration: `compensation_outbox` table.
- No API changes: same 500 semantics, same error envelope, no new payment status.
