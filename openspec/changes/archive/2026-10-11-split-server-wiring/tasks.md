# Tasks

## 1. Split server wiring

- [x] 1.1 Create `internal/app/server.go` with `NewFiberApp` (construction + ErrorHandler), `RegisterMiddlewares` (requestid, Bridge, otelfiber, Stash, RequestLogger, rate limiter in current order), and `RegisterRoutes` (health before limiter-dependent routes, order preserved), and verify `go build ./...` succeeds
- [x] 1.2 Shrink `container.go` to orchestration only (resources, logger/tracer, repos → services → handlers, calls into `server.go`), and verify `go build ./...` + `go vet ./...` + `go test ./...` pass

## 2. Regression

- [x] 2.1 Run live smoke (health, register → pay → duplicate key) against the compose stack, and verify identical codes, headers (`X-Request-ID`, `Idempotency-Key` echo), and a diff of `docker compose` behavior shows nothing new
