# Design

## Context

The service is a Go/Fiber payment gateway with routes defined in `internal/app/server.go` `routes()`: `GET /health`, `GET /ready`, `POST /merchants`, `POST /payments`. Handlers already carry swagger-style `godoc` comments, but nothing generates or serves a machine-readable API definition, and there is no docs route. No OpenAPI spec or Scalar exists in the repo. Constraints: prefer no new Go dependencies; business endpoints only; public `/docs` path.

## Goals / Non-Goals

**Goals:**
- Serve a machine-readable OpenAPI 3.0 definition at `GET /openapi.yaml`.
- Serve Scalar's API reference UI at `GET /docs` pointing at `/openapi.yaml`.
- Document business endpoints only (`POST /merchants`, `POST /payments`).
- Add no new Go dependencies; keep the diff small.

**Non-Goals:**
- Documenting `GET /health` or `GET /ready`.
- Auth-gating or env-flagging the docs route.
- Generating the OpenAPI spec from code or godoc comments.
- Vendoring Scalar for offline use or linking to a hosted Scalar project.
- Changing any existing endpoint behavior or adding runtime dependencies.

## Decisions

**Hand-written OpenAPI YAML at `api/openapi.yaml` (vs. code generation)**
- Rationale: avoids a new dependency and a build step; the spec is small (two endpoints). Code generation from godoc would require a generator not currently in the project.
- Alternative considered: `go-swagger`/similar — rejected, new dependency for a two-endpoint spec.

**Scalar CDN-embed HTML (vs. vendored or hosted)**
- Rationale: a single HTML file embedding Scalar from CDN is the smallest working diff; no static assets to vendor, no external hosting account.
- Alternative considered: vendoring Scalar HTML/JS — rejected, heavier and harder to keep current. Linking out to scalar.com — rejected, not self-contained.

**`GET /docs` always-on and public (vs. env-gated or auth-gated)**
- Rationale: simplest; docs are additive and reveal no secrets. An env flag or auth layer would add config and complexity not requested.
- Alternative considered: gate behind `SCALAR_DOCS=1` — rejected, YAGNI for docs.

## Risks / Trade-offs

- [Scalar UI needs network access to load its CDN JS/CSS] → Offline or air-gapped environments can't render the UI, though `/openapi.yaml` remains available. Mitigation: none for v1; accept the trade-off.
- [Hand-written OpenAPI spec can drift from implementation] → Mitigate by reviewing `api/openapi.yaml` whenever `/merchants` or `/payments` change.
