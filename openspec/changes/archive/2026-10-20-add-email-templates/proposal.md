# Proposal

## Why

Customer-facing emails are currently plain text built inline inside the payment workflow, with no merchant name and no refund context. Adding template-based HTML and plain-text emails makes payment receipts and refund notifications clearer and more useful, while keeping the existing async email-delivery contract intact.

## What Changes

- Add an `internal/email/templates/` directory containing `payment_success` and `refund_in_progress` templates, each in HTML and plain-text form.
- Templates support dynamic fields: payment/refund ID, amount, merchant name, and refund reason.
- Email rendering moves into the `email` package (template text captured now; rendering implementation deferred to a later change).
- `EmailData` gains content-type support so both `text/plain` and `text/html` emails can be sent.
- Refund workflow wiring is deferred to a later change; only template text is captured here.

## Capabilities

### New Capabilities

- None. Template rendering is an implementation detail within the existing `email-notification` delivery contract (publish email event on payment success → background worker delivers async with bounded retries and dead-letter queuing), which is unchanged.

### Modified Capabilities

- None. No requirement text in `email-notification` changes; the delivery contract stays the same, so no delta spec is needed.

## Impact

- `internal/email/` package (sender, new templates, future renderer).
- `internal/payment/workflow/email.go` (future: select template + content type).
- Customer email content: HTML body with merchant name.

This change opts out of specs (`skip_specs: true`) because no spec-level requirement changes.
