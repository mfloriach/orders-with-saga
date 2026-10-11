# Proposal

## Why

`internal/app/container.go` does three jobs — resource setup, object-graph wiring, and Fiber HTTP assembly — making the HTTP layer hard to review or test in isolation. Splitting it clarifies ownership with zero behavior change.

## What Changes

- New `internal/app/server.go` owning all Fiber assembly: app construction, middleware registration, route registration.
- `container.go` keeps orchestration only: open DBs/Redis, build logger/tracer, wire repos → services → handlers, call into `server.go`, return the `Container`.
- Registration order preserved exactly (health stays before the rate limiter, bypassing it as today).

## Capabilities

### New Capabilities

None — pure refactor, no spec-level behavior changes (`skip_specs: true`).

### Modified Capabilities

None.

## Impact

- `internal/app/container.go` (shrinks), `internal/app/server.go` (new). No routes, middleware, deps, or observable behavior change.
