# Tasks

## 1. Pool configuration

- [x] 1.1 Add pool/timeout fields to `internal/config` (PG max open/idle, lifetimes, connect timeout; Redis pool size, dial/read/write timeouts; all defaulted, validated positive) with validation tests, and verify `go test ./internal/config/ -v` passes
- [x] 1.2 Add `internal/app/postgres.go` (open + pool tuning + `PingContext`) and `internal/app/redis.go` (options + ping) wired into `Build`, and verify `go build ./...` passes plus ping failure aborts boot in a stubbed test

## 2. Lifecycle

- [x] 2.1 Add `internal/app/health.go` (`/health` unchanged, new `/ready` with per-dependency checks returning 200/503) and route it in `server.go`, and verify endpoint tests pass in `go test ./internal/app/ -v`
- [x] 2.2 Extend `Container.Shutdown` to close Redis, both DBs, then meter and tracer providers in order, and verify a shutdown test (no leaks, second shutdown safe) passes

## 3. OTel pool metrics

- [x] 3.1 Add `internal/app/telemetry.go` (`MeterProvider` next to tracer, global set, shutdown hook) promoting `otel/metric` to direct, and verify `go build ./...` passes with no new modules in `go.mod`
- [x] 3.2 Add `internal/app/dbmetrics.go` registering observable gauges for `sql.DB.Stats()` (both pools) and `redis.PoolStats()`, and verify gauge registration plus callback values against stubbed stats in `go test ./internal/app/ -v`
