# Design

## Context

See proposal.md — Why. Current state: `payments.status CHECK (status IN ('pending','failed'))` (`migrations/app/002_payments.sql`); `Payment.Status` is a free-form string (`model.go`); transitions are raw SQL literals in `MarkFailed` (`repository_payments.go:67`); `Create` leaves accepted payments `pending` (reconcile-success returns nil without a status write). Crash ambiguity resolves via the existing `GET` mapping in the Mastercard repo (no new endpoint needed).

## Goals / Non-Goals

**Goals:**
- `authorized` (+ reserved `captured`) in the CHECK constraint, marked on confirmed accept.
- Decision-time transition guard with unit tests.
- Batch-readable distinction without outbox joins.

**Non-Goals:**
- Capture execution, refunds, backfill, `authorizing` writes.

## Decisions

**1. CHECK extension via `004_payment_statuses.sql` (`ALTER TABLE ... DROP CONSTRAINT + ADD CONSTRAINT`).**
`('pending','authorized','captured','failed')`. Rationale: Postgres CHECK changes are cheap and lock-safe for this table size; existing rows (`pending`/`failed`) stay valid so no backfill. `captured` is reserved vocabulary with no writer — documented, not enforced-absent.

**2. `MarkAuthorized` mirrors `MarkFailed`.**
`UPDATE payments SET status='authorized' WHERE id=$1 AND status='pending'` with affected-row check (0 rows = already moved or missing → error, no silent no-op). Rationale: same shape as the existing method, same file, same test style; the `pending` precondition makes double-marking fail loudly instead of rewriting history.

**3. Transition table in Go, checked before any write.**
`map[string][]string{"pending": {"authorized","failed"}, "authorized": {"failed","captured"}, "failed": {}, "captured": {}}` behind `func transition(from, to string) error`; repository methods call it first. Rationale: moves the guard from write-time (DB error) to decision-time (typed error, unit-testable without a DB); unknown states fail closed. Alternative (DB-only enforcement) rejected: errors surface late and untyped.

**4. Mark on both accept paths.**
Direct `Call` success and `Status`-confirmed acceptance both mark `authorized` before returning nil from the `call-api` step. Rationale: reconcile-success is otherwise indistinguishable from direct success downstream; marking in one place (the step's success exit) covers both.

**5. No `authorizing` state.**
Crash-during-PUT leaves `pending`; the batch resolves via `GET /payment/<id>` (accepted→mark authorized, absent→fail). Rationale: confirmed exploration decision — saves a hot-path UPDATE per payment at the cost of one network query per stale row.

## Risks / Trade-offs

- [`201` now echoes `authorized` instead of `pending`] → intended contract refinement; handler code unchanged (echoes stored status). Clients keying on `pending` need notice.
- [`captured` writable by nothing] → a writer arriving later must follow the table; the guard already permits `authorized→captured`.
- [Double-mark race (reconcile + retry)] → `pending` precondition + idempotency-conflict path make the loser fail safe (409 with existing payment).

## Migration Plan

1. Land migration `004` + `MarkAuthorized` + guard + tests (no callers yet; inert).
2. Wire marking into the `call-api` success exit; existing payment tests cover regression.
3. Rollback = revert; CHECK extension is backward-compatible with old code (old code only writes `pending`/`failed`).

## Open Questions

None that change specs, approach, or tasks. Capture-endpoint shape belongs to the later capture change.
