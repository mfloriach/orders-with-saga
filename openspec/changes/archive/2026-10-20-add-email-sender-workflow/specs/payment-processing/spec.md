# Spec Delta

## MODIFIED Requirements

### Requirement: Create payment
The system SHALL create a payment with `merchant_id` and positive integer `amount` (minor units, no currency), using the `Idempotency-Key` request header as the idempotency key and deriving one when the header is absent, with initial status `pending`. On success the system sends a synchronous email notification as part of the creation saga after capture, and a failed email send triggers compensation retry (re-send) but does not fail the payment or block the response.

#### Scenario: Successful payment
- **WHEN** client POSTs `/payments` with an existing `merchant_id`, `amount > 0`, and a new `Idempotency-Key` header
- **THEN** system returns `201` with `id`, `merchant_id`, `amount`, `status=pending`, and `created_at`, echoing the key in the `Idempotency-Key` response header

#### Scenario: Invalid payment input
- **WHEN** client POSTs `/payments` with missing/non-positive `amount` or missing `merchant_id`
- **THEN** system returns `400` with code `validation_error`, a `title`, per-field `errors` details, and creates nothing

#### Scenario: Success sends email notification
- **WHEN** payment creation completes with `201`
- **THEN** one email notification is sent synchronously after capture, and email delivery retries on failure via saga compensation without affecting the returned response

## ADDED Requirements

### Requirement: Synchronous email notification after capture
The system SHALL send an email notification as part of payment creation after capture, blocking the payment response until the email send completes or retries are exhausted.

#### Scenario: Email send succeeds
- **WHEN** payment creation reaches the email step after capture and the email send succeeds
- **THEN** creation completes and the API returns `201`

#### Scenario: Email send fails triggers compensation retry
- **WHEN** the email send fails (transient error)
- **THEN** saga compensation re-runs the email step (re-sends), and if retry succeeds, creation completes with `201`

#### Scenario: Email send exhausts retries continues without failing payment
- **WHEN** email send fails and all compensation retries are exhausted
- **THEN** creation completes with `201` (email is best-effort; payment is not marked failed)

#### Scenario: No email configuration skips delivery
- **WHEN** SMTP is not configured
- **THEN** no email is sent and creation continues as if the email step succeeded