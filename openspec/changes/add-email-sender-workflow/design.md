# Design

## Context

Current state (see proposal.md for motivation):
- `internal/payment/workflow/` runs a saga v2 (`pkg/saga/v2`) with steps `authorize → capture → webhook`, wired in `registry.go` via `NewWorkflow(payments, cardNetwork, merchantStore, client)`.
- `internal/config/config.go` loads config from env with validation using `cleanenv` and `validator`.
- `internal/app/container.go` wires dependencies and creates the saga via `workflow.NewWorkflow`.
- Email notification async capability exists (`email-notification` spec) but is decoupled from the payment response via a message broker.

Constraints:
- Email step must not fail the payment on send failure; compensation retries (re-send) but exhausted retries still complete the payment.
- Email step only calls a function (delegates to email service), no direct SMTP logic in workflow.
- SMTP config added to existing config struct with validation at boot.
- No new saga pattern changes; reuse existing `Compensate` re-runs `Do` pattern from `authorize`.

## Goals / Non-Goals

**Goals:**
- Add a synchronous `email` saga step after `capture` (and after `webhook`) that calls an injected email sender function.
- On email send failure, saga compensation re-runs `Do` (re-sends email), mirroring `authorize.Compensate` pattern.
- Add SMTP configuration to `internal/config/config.go` with validation at boot.
- Email sender function injected via `NewWorkflow`; workflow only calls the function.

**Non-Goals:**
- Async email delivery via broker (already exists in `email-notification`).
- Direct SMTP logic in workflow step (must delegate to function).
- Payment failure on email send failure (best-effort).
- Email template management or content generation (out of scope, handled by email service function).
- New config env vars beyond SMTP basics.

## Decisions

| Decision | Rationale | Alternatives considered |
|---|---|---|
| Email step after webhook | Email is a notification after payment completion; webhook notifies merchant, email notifies customer. | Before webhook: could send email before merchant knows. After webhook is more logical. |
| `Compensate` re-runs `Do` (re-send) | Mirrors existing `authorize.Compensate` pattern; minimal new code, consistent compensation style. | No-op compensate: simpler but leaves unsent email un-reconciled. |
| Email sender function injected | Workflow only calls a function; decouples email implementation (SMTP, API, etc.) from saga step. | Direct SMTP in step: couples workflow to SMTP, harder to test and swap. |
| SMTP config in existing config struct | Reuses existing validation and boot-fail-fast pattern; no new config file. | Separate email config file: more complexity, inconsistent with current pattern. |
| Email failure doesn't fail payment | Emails are best-effort notifications; payment success shouldn't depend on email deliverability. | Fail payment on email failure: too strict, blocks legitimate payments on transient email issues. |

## Risks / Trade-offs

- [Email compensation may duplicate sends] Re-running `Do` re-sends email; if the email service isn't idempotent, recipient receives duplicate emails. → Mitigation: include payment ID in email so email service can deduplicate; this matches existing `authorize` compensation risk profile.
- [SMTP config validation at boot] Invalid SMTP config prevents boot, which is fail-fast but means email misconfiguration blocks all traffic. → Mitigation: make SMTP optional (only validate if `SMTP_HOST` is set); warn if missing instead of failing.
- [No email-specific retry classification] Email sender function handles its own retries; saga compensation is the saga-level retry. → Mitigation: document that email sender function should handle transient retries internally; saga compensation is for saga-level recovery.

## Open Questions

- Should the email sender function signature include context for cancellation? Yes, pass `context.Context` for timeout/cancellation.
- Should we add a separate `EmailConfig` struct or inline fields in `Config`? Inline for simplicity, consistent with existing `MC_*` fields.
- What email content/data does the step send? Payment ID, amount, merchant ID, customer email (from payment or merchant). Defer to implementation; function receives workflow data.