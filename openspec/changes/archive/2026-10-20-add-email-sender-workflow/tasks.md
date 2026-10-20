# Tasks

## 1. SMTP Configuration

- [x] 1.1 Add SMTP fields to `internal/config/config.go`: `SMTPHost`, `SMTPPort`, `SMTPUser`, `SMTPPass`, `SMTPTLS`, `SMTPFrom` with env tags, defaults, and validation — verify `go build ./internal/config/`
- [x] 1.2 Add validation logic for SMTP fields (port range, TLS enum) in `config.go` — verify `go test ./internal/config/`
- [x] 1.3 Update `.env.example` with SMTP configuration variables

## 2. Email Sender Function

- [x] 2.1 Create `internal/payment/email/sender.go` with `EmailSender` function type and a default SMTP implementation using `net/smtp` — verify `go build ./internal/payment/email/`
- [x] 2.2 Add unit tests for email sender (success, auth failure, connection failure) — verify `go test ./internal/payment/email/`

## 3. Email Saga Step

- [x] 3.1 Create `internal/payment/workflow/email.go` with the email step: `Name()`, `Do` (calls injected `EmailSender` with workflow data), `Compensate` (re-runs `Do`) — verify `go build ./internal/payment/workflow/`
- [x] 3.2 Add unit tests for the email step (successful send, failed send path, no email config skips) — verify `go test ./internal/payment/workflow/`

## 4. Saga Wiring

- [x] 4.1 Add `emailSender` dependency to `workflow.NewWorkflow` and append the email step after `webhook` in `registry.go` — verify `go build ./internal/payment/workflow/`
- [x] 4.2 Wire the email sender function into `NewWorkflow` in `internal/app/container.go` — verify `go build ./internal/app/`

## 5. Integration Verification

- [x] 5.1 Run `go vet ./...` — verify no errors
- [x] 5.2 Run `go build ./...` — verify the whole module compiles
- [x] 5.3 Run `go test ./...` — verify all tests pass