# Proposal

## Why

The payment gateway exposes business endpoints (`POST /merchants`, `POST /payments`) but has no machine-readable API definition and no developer-facing reference. Operators and integration partners currently have no single source of truth for request/response shapes, headers, or error codes. Adding Scalar gives the service a browsable OpenAPI reference without adding runtime dependencies.

## What Changes

- Add `api/openapi.yaml`: an OpenAPI 3.0 spec documenting the business endpoints (`POST /merchants`, `POST /payments`) with request/response schemas and the `Idempotency-Key` header.
- Add `internal/app/docs.go`: serves the OpenAPI spec at `GET /openapi.yaml` and embeds Scalar's reference UI at `GET /docs` (CDN embed, no new Go dependency).
- Edit `internal/app/server.go`: register `GET /openapi.yaml` and `GET /docs` routes.

## Capabilities

### New Capabilities

- `api-docs`: Serves an OpenAPI spec and Scalar reference UI for the payment gateway's business endpoints, exposing `GET /openapi.yaml` and `GET /docs`. Owns API reference documentation behavior as the surface evolves.

### Modified Capabilities

- None. No existing capability's requirements change; this adds new endpoints and documentation tooling only.

## Impact

- New endpoints: `GET /openapi.yaml`, `GET /docs` (public, always-on).
- New files: `api/openapi.yaml`, `internal/app/docs.go`.
- Edited file: `internal/app/server.go` (two route registrations).
- No new Go dependencies; Scalar UI loaded via CDN from the HTML page.
- No database, broker, or cache changes; no existing endpoint behavior altered.
