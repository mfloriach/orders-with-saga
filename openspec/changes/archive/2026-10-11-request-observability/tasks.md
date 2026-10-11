# Tasks

## 1. Request ID + logging

- [x] 1.1 Add Fiber `requestid` middleware plus `SetUserContext` bridge mapping the ID into Go ctx, and verify responses echo client-supplied IDs and generate unique ones otherwise
- [x] 1.2 Add `pkg/logging` ctx-aware `slog` JSON handler (`request_id`, `trace_id`), inject the logger via the container, and verify a unit test passes (record carries both IDs from ctx)

## 2. Tracing shell + error bodies

- [x] 2.1 Init OTel SDK with no exporter, add `otelfiber` middleware with graceful `Shutdown`, and verify logs for a request contain a valid `trace_id`
- [x] 2.2 Add `request_id` to error bodies in `apperr.Handler` from request ctx, and verify `go test ./...` passes plus live regression (error body `request_id` matches response header)
