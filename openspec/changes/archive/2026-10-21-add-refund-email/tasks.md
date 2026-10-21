# Tasks

## 1. Implement refund email function

- [x] 1.1 Add `RefundInProgress` struct and `SendRefundInProgress` method on `*sMTPSender` in `internal/email/email.go`, rendering `refund_in_progress.html` as HTML to the merchant email with the same error wrapping as `SendPaymentSuccessfully` (verify: `go build ./internal/email/...` succeeds)
- [x] 1.2 Add `SendRefundInProgress` to the `email.Senders` interface (verify: `go vet ./internal/email/...` passes; `sMTPSender` satisfies the interface)

## 2. Verify no regressions

- [x] 2.1 Confirm no other `Senders` implementers exist that need the new method, then run `go build ./...` and `go test ./internal/email/...` (verify: build clean, email tests pass)
