# Design

## Context

No request ID, no structured logging, no OTel deps. Both handlers already pass `c.Context()` into services and repos use `*Context` DB/Redis calls, so ctx-carried IDs propagate with no service changes. See `proposal.md` for why. Constraint: no exporter yet.

## Goals / Non-Goals

**Goals:**
- Per-request `request_id` in header, logs, and error bodies; `trace_id` in logs.
- Exporter-ready: adding an endpoint later is config-only.

**Non-Goals:**
- OTLP exporter, sampling/resource config, collector service, manual service-layer spans, metrics, log shipping.

## Decisions

- **Fiber `requestid` middleware + `SetUserContext` bridge:** native middleware sets the header and `Locals`; one line copies it into Go ctx (`context.WithValue`) so `c.Context()` carries it downstream. Alternative (hand-rolled UUID middleware) rejected: framework already does header + generation.
- **OTel SDK, no span processor:** `TracerProvider` with default (no-op-ish) pipeline + `otelfiber` middleware. Spans are created and ctx-propagated (live `trace_id`s) but dropped at `End`. Alternative (skip OTel entirely until exporter exists) rejected: this keeps the exporter change config-only later. `TracerProvider.Shutdown` wired to process signals anyway, so the exporter change cannot forget it.
- **`pkg/logging` ctx-aware `slog.Handler`:** wraps `slog.JSONHandler`, injects `request_id`/`trace_id` from ctx on every record. Call sites stay `slog.InfoContext(ctx, ...)`. Logger built in `container.Build` and injected into services needing it (manual DI, existing style); also `slog.SetDefault` for middleware-level logging.
- **ErrorHandler reads ctx:** `apperr.Handler` adds `request_id` from `c.UserContext()` to every error body. No handler signature changes.
- **No service/repo code changes:** ctx already flows; only edge wiring + logging package are new.

## Risks / Trade-offs

- [OTel deps with no visible trace output] → value is log correlation + zero-churn exporter later; documented in proposal.
- [`requestid` default header name] → keep `X-Request-ID` default; no customization needed.
- [JSON log volume] → one line per request minimum plus errors; acceptable for this stage.

## Migration Plan

- Deploy: add deps, wire middleware + logger, rebuild. No env or compose changes.
- Rollback: revert binary; headers/bodies revert. No data involved.
- Stage 2 (separate change): OTLP exporter + `OTEL_EXPORTER_OTLP_ENDPOINT`, sampling, collector.
