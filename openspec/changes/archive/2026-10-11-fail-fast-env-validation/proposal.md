# Proposal

## Why

`config.Load()` can never fail: a typo (`APP_DNS`) or missing var silently falls back to localhost defaults, and since `sql.Open` is lazy, a bad DSN only explodes on the first request. The service should refuse to boot on invalid configuration instead.

## What Changes

- Add `github.com/ilyakaznacheev/cleanenv`; tag `Config` with `env`/`env-required`/`env-default`; `Load()` returns `(Config, error)` via `cleanenv.ReadEnv`.
- **BREAKING**: `APP_DSN`, `LEDGER_DSN`, `REDIS_ADDR` become required (no silent localhost fallback). `PORT` keeps default `8080`.
- Expose `RATE_LIMIT` (default `100`) and `RATE_WINDOW_SEC` (default `60`) as env vars with manual `> 0` range checks; invalid port/range fails startup.
- `main()` terminates non-zero with a log line naming the offending var when validation fails.
- Update `.env.example` to mark required vs optional vars.

## Capabilities

### New Capabilities

- `service-config`: startup configuration validation and fail-fast behavior; owns which vars are required, defaults, and boot-failure semantics.

### Modified Capabilities

None — `merchant-registration` and `payment-processing` requirements are unchanged.

## Impact

- `internal/config/config.go`, `cmd/api/main.go`, `go.mod`/`go.sum`, `.env.example`.
- Operators relying on implicit localhost defaults without env set must now export the three DSNs (compose already injects them).
- Explicitly out of scope: connectivity `Ping()` at boot (presence vs reachability; needs retry/backoff — separate change).
