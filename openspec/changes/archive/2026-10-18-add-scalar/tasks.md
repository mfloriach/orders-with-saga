# Tasks

## 1. API spec and docs handler

- [x] 1.1 Write `api/openapi.yaml` documenting `POST /merchants` and `POST /payments` with request/response schemas and the `Idempotency-Key` header — verify the file exists at `api/openapi.yaml` and describes both endpoints
- [x] 1.2 Write `internal/app/docs.go` serving `GET /openapi.yaml` and `GET /docs` with a Scalar CDN-embed HTML page — verify the file exists and the `app` package still compiles

## 2. Route registration and build

- [x] 2.1 Register `GET /openapi.yaml` and `GET /docs` in `internal/app/server.go` `routes()` — verify `go build ./...` succeeds

## 3. Runtime verification

- [x] 3.1 Start the service with dependencies up and verify `GET /docs` returns `200` Scalar HTML that loads `/openapi.yaml`, and `GET /openapi.yaml` returns `200` — verify both endpoints respond
- [x] 3.2 Verify existing business endpoints unchanged — verify `POST /merchants` and `POST /payments` still return their existing status codes and response shapes
