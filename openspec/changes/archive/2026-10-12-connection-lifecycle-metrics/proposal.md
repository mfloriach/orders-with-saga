# Proposal

## Why

`internal/app/container.go` opens two Postgres pools and Redis with library defaults and no verification: `sql.Open` is lazy (first failure hits the first query, not boot), `MaxOpenConns` defaults to unlimited (a burst can exhaust Postgres `max_connections`), Redis has no timeouts, `Shutdown` leaks all three pools (only the tracer flushes), `/health` always reports ok, and pool pressure is invisible. This change gives each dependency a config file, a tuned pool, fail-fast boot, honest lifecycle, and OTel pool metrics.

## What Changes

- Split `internal/app/container.go` wiring into per-type files: `postgres.go` (open + pool + ping), `redis.go` (options + ping), `telemetry.go` (tracer + new meter provider), `dbmetrics.go` (observable gauges), `health.go` (`/health` + `/ready`); `container.go` keeps thin assembly only.
- Pool/timeout env vars via existing cleanenv pattern with validation: PG max open/idle, max lifetime/idle time, connect timeout; Redis pool size, dial/read/write timeouts. Sane defaults (PG 25/5/5m/5s); exact values land in review.
- Fail-fast boot: `Build` pings both PGs + Redis with timeout before returning; failure aborts before binding the port (extends the `service-config` philosophy).
- Lifecycle: `/health` stays dumb liveness; new `/ready` pings dependencies (for load-balancer checks). `Shutdown` closes Redis, both DBs, then meter provider, then tracer flush.
- OTel metrics (per confirmed decision, exporter deferred like tracing): `MeterProvider` next to `TracerProvider`; observable gauges for `sql.DB.Stats()` (open/in-use/idle, wait count/duration, max-idle-closed) and `redis.PoolStats()` (hits/misses/timeouts, total/idle/stale conns). Promotes `otel/metric` indirect→direct; no new modules. Callbacks only run on collection, so cost is ~zero until an exporter lands.
- Stays on `database/sql` (no pgxpool rewrite of repositories).
- Out of scope: exporter config, Prometheus scrape endpoint, runtime (Go) metrics, alerting rules.

## Capabilities

### New Capabilities

None. Lifecycle and telemetry extend existing capabilities.

### Modified Capabilities

- `service-config`: pool/timeout vars with validation, fail-fast dependency ping on boot, `/ready` dependency checks, full pool shutdown.
- `request-observability`: OTel meter provider and registered pool gauge set alongside existing tracing/logging.

## Impact

- New files: `postgres.go`, `redis.go`, `telemetry.go`, `dbmetrics.go`, `health.go` in `internal/app/`; new env vars (all defaulted — minimal-valid-env boot unchanged).
- Modified: `container.go` (assembly), `server.go` (`/ready` route), `config.go` (new fields), `main.go` shutdown path unchanged in shape (richer `Shutdown`).
- No API-contract changes to `/merchants` or `/payments`; `/health` response unchanged.
