# Tasks

## 1. States and guard

- [x] 1.1 Add `migrations/app/004_payment_statuses.sql` extending the CHECK to `('pending','authorized','captured','failed')` and verify it applies cleanly over `001`–`003` on a scratch Postgres
- [x] 1.2 Add the Go transition table (`pending→authorized/failed`, `authorized→failed/captured`, terminal `failed`) with unit tests for every legal and illegal edge, and verify `go test ./internal/payment/ -v` passes

## 2. Authorized marking

- [x] 2.1 Add `MarkAuthorized` to the payment repository (pending-preconditioned update with affected-row check, mirroring `MarkFailed`) with a round-trip test, and verify `go test ./internal/payment/ -v` passes
- [x] 2.2 Mark `authorized` on both accept paths in `Create` (direct call success and status-query reconcile), guarding each write with the transition check, and verify `go test ./...` and `go vet ./...` pass with handler tests updated for the `authorized` echo
