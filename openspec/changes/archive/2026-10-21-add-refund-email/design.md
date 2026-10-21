# Design

## Context

The email package (`internal/email/`) already sends a payment-success email via `SendPaymentSuccessfully` on `*sMTPSender`, rendered from `payment_success.html` through the shared `render()` helper and sent over the pooled SMTP connection in `send()`. A `refund_in_progress.html` (and `.txt`) template already exists and is covered by `sender_test.go`, using fields `RefundID`, `Amount`, `MerchantName`, `RefundReason`. There is no refund email function yet, and no `domain` refund type exists. Per the proposal, this function will not be wired into any workflow, handler, or container yet. (See proposal.md for motivation.)

## Goals / Non-Goals

**Goals:**
- Add `SendRefundInProgress` mirroring `SendPaymentSuccessfully`: render `refund_in_progress.html` as HTML, send to the merchant's email, same error wrapping.
- Add the method to the `email.Senders` interface for consistency with `SendPaymentSuccessfully`.

**Non-Goals:**
- Do not call `SendRefundInProgress` anywhere (workflow, handler, service, container left unchanged).
- Do not introduce a `domain.PaymentRefunded` type or a refund flow.
- Do not change the SMTP sender, renderer, or templates.

## Decisions

- **Struct shape**: `RefundInProgress` holds primitive fields (`RefundID`, `Amount`, `MerchantName`, `RefundReason`) plus `User domain.User`, mirroring `PaymentSuccessfully` but without a nonexistent domain refund type. This keeps the change self-contained; a domain refund type can be added later when the refund flow exists.
- **Interface**: Add `SendRefundInProgress` to `Senders` rather than only `*sMTPSender`, matching how `SendPaymentSuccessfully` is exposed. Only one implementer exists (`sMTPSender`), so no other code breaks.
- **Render/send path**: Reuse the existing `render("refund_in_progress.html", ...)` and `e.send(ctx, emailData{ContentType: "text/html"})` path — no new dependencies, no new TLS/dial logic.

## Risks / Trade-offs

- [Adding a method to `Senders` requires every implementer to adopt it] → Only `sMTPSender` implements `Senders` today; verify no test fakes or other implementers exist before applying.
- [No domain refund type] → The struct uses primitives + `domain.User`, which diverges slightly from `PaymentSuccessfully`'s `domain.PaymentCaptured`. Acceptable until a refund flow and domain type exist; revisit when wiring the function in.
