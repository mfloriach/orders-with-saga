# Tasks

## 1. Repository signatures

- [x] 1.1 Change `PaymentRepository` (`interfaces.go:13-15`) to `MarkFailed/MarkAuthorized/MarkCaptured(ctx, p *Payment)` and verify `go build ./internal/payment/` fails only at expected call sites
- [x] 1.2 Update the three methods in `repositories/payments.go` to assign `p.Status` then run the existing preconditioned `UPDATE`, and verify `go build ./...` passes
- [x] 1.3 Update `repositories/payments_test.go` call sites to pass `p` and assert `p.Status`, and verify `go test ./internal/payment/repositories/ -run TestMark -v` passes (needs `APP_DSN`, skips without it)

## 2. Service call sites

- [x] 2.1 Replace the four `MarkX(ctx, p.ID)` calls in `service.go` with `p`, drop the `TransitionTo` calls and the stray `p.Status = StatusCaptured`, fix compensate `ctx` → `c`, and verify `go build ./...` passes
- [x] 2.2 Run `go vet ./...` and `go test ./internal/payment/... -v`, and verify clean
