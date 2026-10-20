# Tasks

## 1. Template files

- [x] 1.1 Create `internal/email/templates/payment_success.html` with dynamic fields (payment ID, amount, merchant name) — verify: file exists and parses as an `html/template` with `{{.PaymentID}}`, `{{.Amount}}`, `{{.MerchantName}}`
- [x] 1.2 Create `internal/email/templates/payment_success.txt` mirroring the HTML in plain text — verify: file exists and parses as a `text/template` with the same dynamic fields
- [x] 1.3 Create `internal/email/templates/refund_in_progress.html` with dynamic fields (refund ID, amount, merchant name, refund reason) — verify: file exists and parses as an `html/template` with `{{.RefundID}}`, `{{.Amount}}`, `{{.MerchantName}}`, `{{.RefundReason}}`
- [x] 1.4 Create `internal/email/templates/refund_in_progress.txt` mirroring the HTML in plain text — verify: file exists and parses as a `text/template` with the same dynamic fields

## 2. Email content-type support

- [x] 2.1 Add a `ContentType` field to `email.EmailData` (`text/plain` | `text/html`, default `text/plain`) — verify: package compiles and the field defaults to `text/plain`
- [x] 2.2 Update `composeMessage` to set the MIME `Content-Type` header based on `ContentType` — verify: `text/html` is emitted when `ContentType` is `text/html`, `text/plain` otherwise

## 3. Verification

- [x] 3.1 Add a unit test that parses and renders each template with sample data — verify: `go test ./internal/email/...` passes and each template renders without error

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.
- Verify the archived result.
