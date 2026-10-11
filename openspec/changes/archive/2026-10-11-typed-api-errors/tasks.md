# Tasks

## 1. Typed errors + central handler

- [x] 1.1 Define the 6 structs with status mapping and the Fiber `ErrorHandler` in `pkg/apperr` (extras as fields, `details` defaults to `title`), and verify a rendering table unit test passes for every type (status + JSON shape)
- [x] 1.2 Migrate merchant + payment handlers to return typed errors/sentinels, delete `Write()`, and verify `go build ./...` succeeds (deletion makes stragglers compile errors)

## 2. Fold-in + regression

- [x] 2.1 Fold in the payment-409 (`payment_id`) and limiter-429 (`retry_after`) outliers, register the ErrorHandler in `container.go` with Fiber-default passthrough for unknown errors, and verify `go vet ./...` passes
- [x] 2.2 Run regression against the compose stack (register → pay → dup-key → bad-input → rate-limit), and verify all 6 status codes are unchanged and each error body carries `code`, `title`, and its extras

## 3. Error-constructor + handler tests (follow-up)

- [x] 3.1 Add missing `MerchantNotFound` / `RateLimitedError` constructors (spec codes) and verify `go test ./pkg/apperr/` passes unmodified
- [x] 3.2 Add handler error-path tests via stub services through the central ErrorHandler, and verify `go test ./...` passes (all packages)
