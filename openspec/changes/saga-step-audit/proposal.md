# Proposal

## Why

The payment audit trail (table, repository, spec) exists but nothing writes to it: `saga.Run` is opaque to the service, so per-step outcomes are unobservable and the specified audit rows are never recorded.

## What Changes

- `pkg/saga` gains a generic step-observer hook (`OnStep` + `WithOnStep` option) invoked synchronously after each executed step's `Do`, with the item, step name, and result.
- `payment.Service.Create` builds the saga per request (instead of once at construction) and wires the hook to `AuditStore.RecordStep`, recording one row per executed step with payment id, saga id, outcome, and reason.
- Per-request saga construction also gives each payment its own saga uuid (today one id is shared by all payments).
- An audit-write failure logs loudly and never fails the payment.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `payment-audit-trail`: rows are recorded via the saga step hook, including the guarantee that an audit-write failure never fails the payment.
- `saga-library`: the saga exposes a synchronous per-step observer hook.

## Impact

- `pkg/saga`: new hook field + option, one invocation point in the `Run` loop.
- `internal/payment`: service gains an `audit` field (`domain.AuditStore`, already defined) and a `NewService` param; container passes the existing `AuditRepository`.
- No API, migration, or topology changes. No behavior change to success/failure responses.
