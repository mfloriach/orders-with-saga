# Design

## Context

`pkg/apperr` is `Write(c, status, code, errs)` rendering `{code, errors?}`; 6 call sites pass raw strings and 2 spots (payment 409, limiter 429) bypass it with `fiber.Map`. See `proposal.md` for why. Constraint from explore: central Fiber ErrorHandler, only types with callers today.

## Goals / Non-Goals

**Goals:**
- One render path for every error body; handlers return errors, never status codes.
- Extras (`payment_id`, `retry_after`) as first-class struct fields, not ad-hoc maps.
- `code` values and statuses byte-identical to today.

**Non-Goals:**
- `Unauthorized`/`Forbidden` types (no auth in v1), machine-readable error docs/OpenAPI, request-ID correlation.

## Decisions

- **Typed structs with `Status()` + central `ErrorHandler`:** `BadRequest`, `NotFound`, `Conflict{PaymentID}`, `RateLimited{RetryAfter}`, `Internal`, `ValidationError{Errors}`. Each struct exposes its status; `fiber.New(ErrorHandler: apperr.Handler)` switches on type and renders `{code, title, details, ...omitted empties}`. Alternative (typed values passed to `Write` at call sites) rejected: preserves today's scatter.
- **Sentinels stay as the service→handler contract:** `merchant.ErrEmailTaken`, `merchant.ErrNotFound`, `payment.ConflictError` keep services HTTP-free; the ErrorHandler (or a tiny `Map(err)` helper it calls) translates them to typed structs. Services unchanged.
- **Delete `Write()`:** removes the raw-string path so new code cannot backslide; compiler enforces migration completeness.
- **`details` defaults to `title`:** callers pass details only when they add information; renderer fills blanks. Avoids `details: ""` noise and per-site boilerplate.
- **Rate limiter returns `&apperr.RateLimited{}`:** middleware already returns errors, so it fits the handler without restructuring.

## Risks / Trade-offs

- [New JSON fields on error bodies] → additive only; tolerant clients unaffected. Verified by regression task.
- [Fiber ErrorHandler also receives non-apperr errors (e.g. Fiber's own 404/405)] → default branch renders `Internal` 500? No — pass through Fiber's default for unknown errors to avoid masking; only apperr/sentinel types are mapped, everything else delegates to `fiber.DefaultErrorHandler`.
- [Six structs for six call sites looks 1:1] → the win is the closed set + extras as fields, not line count.

## Migration Plan

- Land apperr + handler migration + limiter fold-in atomically (single change); `Write` deletion makes partial migration a compile error, so no phased rollout needed.
- Rollback: revert binary; response shape reverts to `{code, errors?}`/maps. No data involved.
