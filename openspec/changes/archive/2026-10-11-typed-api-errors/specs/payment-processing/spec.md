# Spec Delta

## MODIFIED Requirements

### Requirement: Create payment
The system SHALL create a payment with `merchant_id`, positive integer `amount` (minor units, no currency), and required `idempotency_key`, with initial status `pending`.

#### Scenario: Successful payment
- **WHEN** client POSTs `/payments` with an existing `merchant_id`, `amount > 0`, and new `idempotency_key`
- **THEN** system returns `201` with `id`, `merchant_id`, `amount`, `status=pending`, and `created_at`

#### Scenario: Invalid payment input
- **WHEN** client POSTs `/payments` with missing/non-positive `amount`, missing `merchant_id` or `idempotency_key`
- **THEN** system returns `400` with code `validation_error`, a `title`, per-field `errors` details, and creates nothing

### Requirement: Payment idempotency
The system SHALL return the original payment when the same `merchant_id` + `idempotency_key` is reused, without creating a duplicate.

#### Scenario: Duplicate idempotency key
- **WHEN** client POSTs `/payments` with a previously used `merchant_id` + `idempotency_key`
- **THEN** system returns `409` with the original `payment_id`, a `title`, and `details`, and creates no new payment or ledger entry

#### Scenario: Same key different merchant
- **WHEN** two different merchants use the same `idempotency_key`
- **THEN** system treats them as distinct and creates two payments

### Requirement: Rate limiting
The system SHALL rate-limit payment and merchant endpoints per client IP using Redis.

#### Scenario: Rate limit exceeded
- **WHEN** a client exceeds 100 requests/minute per IP
- **THEN** system returns `429` with a `retry_after` hint, a `title`, and `details`
