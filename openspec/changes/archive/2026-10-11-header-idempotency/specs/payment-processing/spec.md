# Spec Delta

## MODIFIED Requirements

### Requirement: Create payment
The system SHALL create a payment with `merchant_id` and positive integer `amount` (minor units, no currency), using the `Idempotency-Key` request header as the idempotency key and deriving one when the header is absent, with initial status `pending`.

#### Scenario: Successful payment
- **WHEN** client POSTs `/payments` with an existing `merchant_id`, `amount > 0`, and a new `Idempotency-Key` header
- **THEN** system returns `201` with `id`, `merchant_id`, `amount`, `status=pending`, and `created_at`, echoing the key in the `Idempotency-Key` response header

#### Scenario: Invalid payment input
- **WHEN** client POSTs `/payments` with missing/non-positive `amount` or missing `merchant_id`
- **THEN** system returns `400` with code `validation_error`, a `title`, per-field `errors` details, and creates nothing

### Requirement: Payment idempotency
The system SHALL return the original payment when the same `merchant_id` + idempotency key is reused, without creating a duplicate. A missing `Idempotency-Key` header derives the key as `drv:` + hex `SHA-256(merchant_id:amount)` with a short window (~10 min); explicit client keys keep a 24h window.

#### Scenario: Duplicate idempotency key
- **WHEN** client POSTs `/payments` with a previously used `merchant_id` + `Idempotency-Key` header
- **THEN** system returns `409` with the original `payment_id`, a `title`, and `details`, and creates no new payment or ledger entry

#### Scenario: Same key different merchant
- **WHEN** two different merchants use the same `Idempotency-Key` header
- **THEN** system treats them as distinct and creates two payments

#### Scenario: Missing header derives key
- **WHEN** client POSTs `/payments` without an `Idempotency-Key` header
- **THEN** the system derives the key from the request, processes idempotently, and echoes the derived key in the response header
