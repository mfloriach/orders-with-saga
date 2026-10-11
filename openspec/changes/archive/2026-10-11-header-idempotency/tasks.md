# Tasks

## 1. Header key + derivation

- [x] 1.1 Add `ResolveKey` helper (header-or-derive with `drv:` prefix, >200-char fallback) with a unit test, and verify `go test ./internal/payment/` passes (explicit, missing, oversized cases)
- [x] 1.2 Migrate handler (header read, body struct shrinks, echo response header), thread per-key TTL through the service, and verify `go build ./...` + `go vet ./...` pass

## 2. Tests + regression

- [x] 2.1 Extend handler tests (header key, missing header derives + echoes, duplicate via header → 409) and verify `go test ./...` passes
- [x] 2.2 Run live regression (header payment 201, repeat 409, no-header payment derives + echoes, unknown merchant 404), and verify codes, echo headers, and Redis TTLs (24h vs ~10m)

## 3. Service-side resolution (follow-up)

- [x] 3.1 Move key resolution into `Service.Create` (raw header in, TTL internal), echo from the payment object, update handler tests, and verify `go test ./...` passes plus live regression (codes, echoes, both TTLs)
