# Proposal

## Why

Error codes are raw strings at 6 call sites with nothing enforcing the set, and 2 endpoints bypass `apperr` with ad-hoc `fiber.Map` bodies because the generic `{code, errors?}` envelope cannot carry extras like `payment_id` or `retry_after`. Typed errors plus one central renderer fix both.

## What Changes

- Add typed error structs in `pkg/apperr` — `BadRequest`, `NotFound`, `Conflict`, `RateLimited`, `Internal`, `ValidationError` — each carrying `code`, `title`, `details` (plus `errors` map on `ValidationError`, `payment_id` on `Conflict`, `retry_after` on `RateLimited`). No `Unauthorized`/`Forbidden` (no auth in v1; add with that change).
- Add a central Fiber `ErrorHandler`: handlers return typed errors (or existing sentinels), rendering happens in one place. Handlers stop naming HTTP status codes.
- Migrate all 6 `apperr.Write` call sites plus the 2 `fiber.Map` outliers; delete `Write()` to prevent backsliding.
- `code` values and statuses unchanged (`email_taken` 409, `merchant_not_found` 404, `idempotency_conflict` 409, `validation_error` 400, `rate_limited` 429); `title`/`details` are additive.

## Capabilities

### New Capabilities

None — the error contract already exists; this change reshapes its envelope.

### Modified Capabilities

- `merchant-registration`: error scenarios gain `title`/`details` on `validation_error` 400 and `email_taken` 409.
- `payment-processing`: error scenarios gain `title`/`details` on `validation_error` 400, `merchant_not_found` 404, `idempotency_conflict` 409 (with `payment_id`), and `rate_limited` 429 (with `retry_after`).

## Impact

- `pkg/apperr/apperr.go`, `internal/merchant/handler.go`, `internal/payment/handler.go`, `internal/ratelimit/ratelimit.go`, `internal/app/container.go` (register ErrorHandler). No DB, routes, or dependencies change. Additive JSON fields only — tolerant clients unaffected.
