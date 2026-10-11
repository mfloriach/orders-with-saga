# Proposal

## Why

Requests are untraceable: no request ID, no structured logs (only `log.Fatal` in `main`), and no tracing. When a merchant reports a failed payment there is nothing to correlate it with. Handlers already propagate `c.Context()` into services, so edge-injected IDs flow everywhere for free.

## What Changes

- Add Fiber `requestid` middleware: every response carries `X-Request-ID` (honoring a client-supplied value), mapped into Go ctx so downstream code sees it.
- Add stdlib `slog` JSON logging to stdout with a small ctx-aware handler injecting `request_id` (and `trace_id` when present); logger injected via the manual DI container.
- Initialize the OTel SDK with **no exporter**: per-request server spans via `otelfiber` give live `trace_id`s for log correlation, spans are dropped locally. Exporter, sampling, and collector are an explicit later change.
- Echo `request_id` in error response bodies (typed-error envelope).

## Capabilities

### New Capabilities

- `request-observability`: request ID propagation, structured logging, and trace-ID log correlation.

### Modified Capabilities

None — existing endpoint behaviors are unchanged; only additive response fields/headers.

## Impact

- New deps: OTel SDK + `otelfiber` (no exporter packages). `internal/app/container.go`, `pkg/apperr` (request_id field), new `pkg/logging`, `docker-compose.yml` untouched, no new services.
- Observable surface: `X-Request-ID` response header + `request_id` in error bodies (additive only).
- Explicitly out of scope: OTLP exporter/endpoint, sampling config, collector service, manual service-layer spans, metrics.
