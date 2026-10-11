# Design

## Context

`internal/config/config.go` is a 30-line `os.Getenv`-with-defaults helper; `Load()` returns `Config` (no error) and `cmd/api/main.go` proceeds unconditionally. See `proposal.md` for why. Constraint: use `github.com/ilyakaznacheev/cleanenv`.

## Goals / Non-Goals

**Goals:**
- Missing/invalid env fails the process at boot with the offending var named in the log.
- `RATE_LIMIT` / `RATE_WINDOW_SEC` become env-configurable without changing current defaults.

**Non-Goals:**
- Connectivity checks (`Ping`) at boot — separate change.
- `.env` file autoloading — compose/docker injects env; avoids "which source won?" ambiguity. cleanenv supports it later via `ReadConfig` if needed.

## Decisions

- **cleanenv `ReadEnv` (env only):** struct tags (`env`, `env-required`, `env-default`) become the documentation; `GetDescription` available for `--help` later. Alternative (hand-rolled `os.Getenv` + checks) rejected: same code without tag-docs or future `.env` support.
- **`Load() (Config, error)`:** returns error instead of `log.Fatal`-ing inside the package — keeps validation unit-testable. `main()` does `log.Fatalf("invalid config: %v", err)`; cleanenv's error already names the field/var.
- **Manual range checks post-load:** cleanenv validates presence/format, not ranges — 5 lines in `config` package (`PORT` 1–65535 numeric, `RateLimit > 0`, `RateWindow > 0`). `PORT` stays a string in `Config` (used as `":"+cfg.Port` in `Listen`); parse only for validation.
- **`container.go` untouched** except receiving the same `Config` type; `RateLimit`/`RateWindow` flow into the existing limiter wiring instead of hardcoded `100`/`60`.

## Risks / Trade-offs

- [Local-dev friction: 3 vars now mandatory] → `.env.example` updated; compose already injects them.
- [cleanenv error text is field-name based] → fine, we only log it; not machine-parsed.
- [New dependency for ~30 lines of config] → justified by tag-docs, `.env` path, and named-var errors; tiny single-purpose lib.

## Migration Plan

- Deploy: `go get` cleanenv, update env (ensure 3 DSNs exported), rebuild. No data migration.
- Rollback: revert binary; old binary boots with defaults as before.
