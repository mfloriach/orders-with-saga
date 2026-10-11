# Tasks

## 1. Config validation

- [x] 1.1 Add cleanenv dependency, re-tag `Config` (required DSNs, defaulted port/limits), change `Load()` to return `(Config, error)`, and verify `go build ./...` succeeds
- [x] 1.2 Add range validation (numeric port 1-65535, `RateLimit > 0`, `RateWindow > 0`) with a table unit test, and verify `go test ./internal/config/` passes (valid, missing DSN, bad port, zero limit cases)
- [x] 1.3 Wire `main()` fail-fast (`log.Fatalf` on `Load` error), thread `RateLimit`/`RateWindow` into container wiring, update `.env.example`, and verify `go vet ./...` passes

## 2. Verification

- [x] 2.1 Verify fail-fast behavior: boot without `APP_DSN` exits non-zero naming `APP_DSN` with no port bound; `RATE_LIMIT=0` and `PORT=abc` each exit non-zero naming the var
- [x] 2.2 Run regression: boot with full env serves `GET /health`, and compose smoke (register → pay) returns the same codes as before, and verify `go build` + smoke pass
