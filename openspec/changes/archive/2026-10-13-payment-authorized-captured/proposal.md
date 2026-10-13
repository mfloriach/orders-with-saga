# Proposal

## Why

`payments.status` knows only `pending` and `failed`, yet the system now produces three distinct realities: freshly created, network-accepted (funds held), and in-doubt (outcome unknown). All three read as `pending`, so any future batch, settlement, or refund logic faces archaeology (`pending AND NOT in outbox AND old AND ...`) instead of a state read. The card flow is authorize-then-capture; the vocabulary needs to say so before capture execution lands.

## What Changes

- Extend `payments.status` CHECK to `('pending','authorized','captured','failed')` via new migration; `captured` has no writer yet (capture execution is a later change).
- Mark `authorized` on confirmed network accept (reconcile-success path included); `failed` and `pending`/in-doubt semantics unchanged.
- Add a Go transition guard (`pending→authorized`, `pending→failed`, `authorized→failed`, `authorized→captured`, `failed→∅`) so illegal transitions fail at decision time instead of at the DB.
- Crash-during-PUT ambiguity resolves via `GET /payment/<id>` (no `authorizing` writes; confirmed exploration decision).
- Out of scope: capture execution (saga step/endpoint driving `authorized→captured`), refunds/settlement reads, backfill (existing rows are only `pending`/`failed`, both still valid).

## Capabilities

### New Capabilities

None. States extend existing payment behavior.

### Modified Capabilities

- `payment-processing`: new authorized-state requirement (marking on accept, transition rules, batch-readable distinction from in-doubt `pending`).

## Impact

- New migration `migrations/app/004_payment_statuses.sql`; new `MarkAuthorized` repository method; transition guard in payment package with unit tests.
- Modified: `Create` reconcile-success path (mark authorized instead of leaving pending), `MarkFailed` call sites unchanged.
- API responses now surface `authorized` where they previously echoed `pending` for accepted payments (intended, recorded).
