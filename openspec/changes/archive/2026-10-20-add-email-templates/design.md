# Design

## Context

Email is sent synchronously inside the payment workflow by `emailStep` (`internal/payment/workflow/email.go`), which builds a plain-text body with `fmt.Sprintf` and hands `EmailData{To, Subject, Body}` to `email.SMTPSender`. `SMTPSender.Send` runs `composeMessage`, which hardcodes `Content-Type: text/plain` (`internal/email/sender.go:172`). SMTP config (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_TLS`, `SMTP_FROM`) comes from `internal/config` and is wired in `internal/app/container.go`. Go 1.27 is in use, so `//go:embed` is available with no new dependency. The `email-notification` spec already defines the delivery contract: publish an email event on payment success, deliver async from a background worker with bounded retries and dead-letter queuing. That contract is unchanged by this change.

## Goals / Non-Goals

**Goals:**
- Capture `payment_success` and `refund_in_progress` email template text (HTML + plain text) in `internal/email/templates/`.
- Support dynamic fields: payment/refund ID, amount, merchant name, refund reason.
- Allow both `text/plain` and `text/html` content types on outgoing email.
- Keep template rendering inside the `email` package.

**Non-Goals:**
- Rendering implementation (deferred to a later change).
- Refund workflow wiring (deferred to a later change).
- Changing the async delivery contract (publish → worker → retries/DLQ).

## Decisions

**1. Template storage: HTML + plain-text files under `internal/email/templates/`, embedded with `//go:embed`.**
- Rationale: keeps HTML editable as real files, avoids embedding HTML in Go string literals, and `//go:embed` is stdlib in Go 1.27 (no new dependency).
- Alternatives considered: Go string constants (awkward HTML editing), a third-party template engine (unneeded dependency). `html/template`/`text/template` from stdlib is sufficient for variable substitution.

**2. `EmailData` gains a content-type field (`text/plain` | `text/html`) alongside the body.**
- Rationale: callers can choose plain text or HTML per template; the sender sets the MIME content type accordingly instead of hardcoding `text/plain`.
- Alternative: always send `text/html` — rejected, would break existing plain-text callers.

**3. Template rendering lives in the `email` package (a `TemplateRenderer`), not in the workflow step.**
- Rationale: keeps the workflow step thin and centralizes email formatting in one package, matching the user's request.

**4. Refund workflow wiring is deferred; only template text is captured in this change.**
- Rationale: refunds are not a current capability; wiring them into a workflow is implementation work for a later change.

## Risks / Trade-offs

- [HTML escaping mistakes could inject malformed markup] → Use `html/template` (auto-escaping) for HTML templates and `text/template` for plain text; review rendered output in tests.
- [Template and Go code drift out of sync] → Co-locate templates next to the sender and add a test that renders each template with sample data.
- [Deferred rendering means templates are text-only for now] → Accepted per scope; rendering is a well-bounded follow-up task.

## Migration Plan

No runtime migration: this change adds template files only and alters no running behavior. When rendering is implemented later, the workflow step will select the template + content type and the sender will set the MIME type accordingly; rollback is simply not wiring the renderer.
