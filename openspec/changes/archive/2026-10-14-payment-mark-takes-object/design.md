# Design

## Context

See proposal.md - Why. Current state: `service.go` owns the in-memory transition (`p.TransitionTo(...)` before each `MarkX(ctx, p.ID)` call, plus a redundant `p.Status = StatusCaptured` after `MarkCaptured`), while `payments.go` owns the SQL `UPDATE ... WHERE status = <expected>` guard. Two writers, two vocabularies, one status.

## Goals / Non-Goals

**Goals:**
- One writer per status change: repo assigns `p.Status` and persists it.
- Keep the SQL `WHERE` precondition as the single transition guard.

**Non-Goals:**
- No `TransitionTo` refactor, no new statuses, no ledger/audit changes (`AuditStore` untouched).
- No change to saga flow, in-doubt handling, or API responses.

## Decisions

1. **Mutate-then-write, no `TransitionTo` in repo.** Each `MarkX(ctx, p)` assigns `p.Status = <target>` first, then runs the existing preconditioned `UPDATE`. Alternative (DB-first-then-assign) keeps `p` truthful on failure but was rejected per exploration decision. Alternative (repo calls `TransitionTo`) rejected — dual guards drift; SQL `WHERE` already rejects illegal moves via the affected-row check.
2. **`AuditStore.MarkFailed` unchanged.** It marks a `ledger_entries` row, not a payment — passing `*Payment` would conflate domains.
3. **Fix compensate `ctx` → `c`** at the `MarkFailed` call site in the same pass (uses the saga step's context).

## Risks / Trade-offs

- [Risk] On DB error `p` claims an unpersisted status → callers must treat `p` as dirty after error (all current call sites return immediately on error, so contained).
- [Risk] In-memory assignment and SQL literal could disagree on the target string → Mitigation: both are adjacent lines in the same method; tests cover the round-trip.
