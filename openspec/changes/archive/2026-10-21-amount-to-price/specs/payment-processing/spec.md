# Spec Delta

## MODIFIED Requirements

### Requirement: Create payment
The system SHALL create a payment with `merchant_id` and a `price` object (`amount` in minor units plus `coin` currency), using the `Idempotency-Key` request header as the idempotency key and deriving one when the header is absent, with initial status `pending`. On success the system sends a synchronous email notification as part of the creation saga after capture, and a failed email send triggers compensation retry (re-send) but does not fail the payment or block the response.

#### Scenario: Successful payment
- **WHEN** client POSTs `/payments` with an existing `merchant_id`, a `price` object with `amount > 0` and `coin`, and a new `Idempotency-Key` header
- **THEN** system returns `201` with `id`, `merchant_id`, `price {amount, coin}`, `status=pending`, and `created_at`, echoing the key in the `Idempotency-Key` response header

#### Scenario: Invalid payment input
- **WHEN** client POSTs `/payments` with missing/invalid `price` (non-positive `amount` or missing `coin`) or missing `merchant_id`
- **THEN** system returns `400` with code `validation_error`, a `title`, per-field `errors` details, and creates nothing

#### Scenario: Success sends email notification
- **WHEN** payment creation completes with `201`
- **THEN** one email notification is sent synchronously after capture, and email delivery retries on failure via saga compensation without affecting the returned response
