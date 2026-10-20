# Proposal

## Why

The payment workflow needs to send synchronous email notifications (e.g., payment receipts) as part of the saga flow after a payment is captured. Currently, the system only supports async email delivery via a message broker (email-notification capability), which does not block the payment response. Merchants and customers require synchronous email delivery for critical notifications like receipts, where the email must be sent before the payment response completes. A failed email send should retry (via saga compensation) rather than fail the payment.

## What Changes

- Add a new `email` saga step in `internal/payment/workflow/email.go` after `capture` that sends an email using a function call (delegating to an email service).
- The email step's `Compensate` re-runs `Do` (re-sends the email), mirroring the existing `authorize` compensation pattern.
- Add SMTP configuration to `internal/config/config.go` with validation at boot.
- Wire an email sender function into `workflow.NewWorkflow` so the step can send emails.
- A failed email step retries via compensation; the payment is NOT marked failed for email errors (emails are best-effort within the saga).

## Capabilities

### New Capabilities

None. The behavior (synchronous email sending as part of payment workflow) is an extension of existing payment-processing capability; no new durable capability boundary is introduced.

### Modified Capabilities

- `payment-processing`: the payment creation saga now includes a synchronous email sending step after capture. Email failure triggers compensation retry (re-send), but does not fail the payment or block the response.
- `service-config`: new SMTP configuration fields added with validation at boot.

## Impact

- `internal/payment/workflow/email.go` (new): the email step implementation.
- `internal/payment/workflow/registry.go`: new `email` step appended to the saga; `NewWorkflow` gains an email sender function dependency.
- `internal/config/config.go`: new SMTP configuration fields with validation.
- `internal/app/container.go`: wire the email sender function into `NewWorkflow`.
- Openspec deltas: `payment-processing` and `service-config` requirement changes.