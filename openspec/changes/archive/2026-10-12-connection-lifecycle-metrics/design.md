# Design

## Context

See proposal.md — Why. Current state: `internal/app/container.go:32-44` opens both PGs via bare `sql.Open("pgx", dsn)` and Redis via `redis.NewClient` with zero tuning or verification; `Container.Shutdown` flushes only the tracer (`container.go:28-30`); `server.go:33` serves a static `/health`; `main.go:32-40` stops traffic then calls `Shutdown`. Config uses cleanenv with fail-fast validation (`internal/config/config.go`). OTel SDK (tracer, no exporter) is initialized in `pkg/tracing`; `otel/metric` v1.47.0 is already in the module graph as indirect. go-redis v9.23.0 exposes `Client.PoolStats()` with hits/misses/timeouts/total/idle/stale.

## Goals / Non-Goals

**Goals:**
- Per-dependency dial files with tuned pools, boot ping, deep health, full shutdown, OTel pool gauges.
- Zero new modules; exporter deferred exactly like tracing.

**Non-Goals:**
- Exporter config, Prometheus endpoint, Go runtime metrics, pgxpool migration, alerting.

## Decisions

**1. File split by dependency, thin assembly.**
`postgres.go` owns open + `SetMaxOpenConns/SetMaxIdleConns/SetConnMaxLifetime/SetConnMaxIdleTime` + `PingContext`; `redis.go` owns `redis.Options` (pool size, timeouts) + `Ping`; `telemetry.go` owns tracer + meter providers; `dbmetrics.go` owns gauge registration; `health.go` owns `/health` + `/ready`. Rationale: matches the request and keeps `container.go` as pure assembly. Alternative (one `deps.go`) rejected: re-creates the file being split.

**2. Env vars with cleanenv validation, all defaulted.**
`PG_MAX_OPEN=25`, `PG_MAX_IDLE=5`, `PG_MAX_LIFETIME_SEC=300`, `PG_MAX_IDLE_SEC=60`, `PG_CONNECT_TIMEOUT_SEC=5`; `REDIS_POOL_SIZE` (default 0 = go-redis per-CPU default), `REDIS_DIAL_TIMEOUT_SEC=5`, `REDIS_READ_TIMEOUT_SEC=3`, `REDIS_WRITE_TIMEOUT_SEC=3`. Non-positive pool sizes and negative durations fail validation naming the field. Rationale: extends the existing fail-fast pattern; defaults preserve minimal-env boot per the `service-config` minimal-valid-env scenario.

**3. Ping in `Build` with per-dependency timeout.**
Each ping wrapped in `context.WithTimeout`; first failure aborts `Build` with a naming error before any port binds. Rationale: consistent with fail-fast config philosophy; docker-compose already orders via healthchecks so local boot is unaffected.

**4. `/ready` separate from `/health`.**
`/health` byte-identical (liveness); `/ready` pings AppDB, LedgerDB, Redis with short timeout and returns 200 with per-dep status or 503 naming failures. Rationale: load balancers distinguish "alive" from "able to serve"; deepening `/health` would change its contract.

**5. OTel observable gauges, exporter deferred.**
`telemetry.go` builds `sdkmetric.NewMeterProvider` (same module as the existing tracer provider — no new deps) and sets it global; `dbmetrics.go` registers `Float64ObservableGauge`s with callbacks reading `db.Stats()` and `PoolStats()`. Shutdown order: stop traffic → close Redis/DBs → `MeterProvider.Shutdown` → `TracerProvider.Shutdown`. Rationale: mirrors the tracing no-exporter posture; callbacks only run on collection so idle cost is ~zero. Rejected Prometheus client (new dep) and stdlib expvar (user chose OTel).

**6. Stay on `database/sql`.**
pgxpool has finer knobs but every repository takes `*sql.DB`; migration is a separate change. Stdlib pool knobs cover the exhaustion risk (the unbounded-`MaxOpenConns` default is the actual bug).

## Risks / Trade-offs

- [Ping couples boot to DB availability] → matches existing fail-fast stance; compose healthchecks cover local boot. Accepted.
- [Gauges invisible until exporter lands] → same accepted posture as tracing; `/ready` gives immediate operational signal meanwhile.
- [Pool defaults are guesses] → values land in review; gauges make mistuning observable post-deploy.

## Migration Plan

1. Land config fields + dial files + ping (boot behavior change; existing config tests cover validation).
2. Land `/ready`, shutdown order, meter provider + gauges.
3. Rollback = revert; no migrations, no contract changes to business endpoints.

## Open Questions

None that change specs, approach, or tasks. Exact pool numbers resolve in review.
