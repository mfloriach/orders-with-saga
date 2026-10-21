# Proposal

## Why

The payment gateway sends a payment-success email but has no email path when a refund is initiated. Merchants need to be notified that a refund is in progress, using the already-authored `refund_in_progress.html` template, without blocking the payment response path.

## What Changes

- Add a `RefundInProgress` email function to the email sender package, mirroring `SendPaymentSuccessfully` (same renderer, same SMTP sender, same `ContentType: text/html` handling).
- Add `SendRefundInProgress` to the `email.Senders` interface so callers can use it polymorphically (consistent with `SendPaymentSuccessfully`).
- Add a refund email notification requirement to the `email-notification` spec capability.
- Do not wire the function into any workflow, handler, or container; it will not be called yet.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `email-notification`: Add a requirement that the system sends a refund-in-progress email carrying refund id, amount, merchant name, and refund reason when a refund is initiated.

## Impact

- `internal/email/email.go` — new `RefundInProgress` struct and `SendRefundInProgress` method on `*sMTPSender`; new method on `Senders` interface.
- `internal/email/templates/refund_in_progress.html` — already exists; rendered with `RefundID`, `Amount`, `MerchantName`, `RefundReason`.
- No workflow, handler, service, or container changes in this change.
