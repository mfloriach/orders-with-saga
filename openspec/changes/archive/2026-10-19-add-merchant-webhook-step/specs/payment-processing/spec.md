# Spec Delta

## MODIFIED Requirements

### Requirement: Create payment
The system SHALL create a payment with `merchant_id` and positive integer `amount` (minor units, no currency), using the `Idempotency-Key` request header as the idempotency key and deriving one when the header is absent, with initial status `pending`. On success the system calls the merchant's registered `webhook_url` synchronously as part of creation, and a failed webhook call marks the payment `failed` and compensates prior steps.

#### Scenario: Successful payment
- **WHEN** client POSTs `/payments` with an existing `merchant_id`, `amount > 0`, and a new `Idempotency-Key` header
- **THEN** system returns `201` with `id`, `merchant_id`, `amount`, `status=pending`, and `created_at`, echoing the key in the `Idempotency-Key` response header

#### Scenario: Invalid payment input
- **WHEN** client POSTs `/payments` with missing/non-positive `amount` or missing `merchant_id`
- **THEN** system returns `400` with code `validation_error`, a `title`, per-field `errors` details, and creates nothing

#### Scenario: Success publishes notification events
- **WHEN** payment creation completes with `201`
- **THEN** one webhook event and one email event for that payment are published, and neither delivery affects the returned response

## ADDED Requirements

### Requirement: Synchronous merchant webhook notification
The system SHALL call the merchant's registered `webhook_url` as part of payment creation, blocking the payment response until the call completes.

#### Scenario: Webhook call succeeds
- **WHEN** payment creation reaches the webhook step and the merchant's `webhook_url` returns `2xx`
- **THEN** creation completes and the API returns `201`

#### Scenario: Webhook call fails compensates prior steps
- **WHEN** the merchant's `webhook_url` returns non-2xx or the call fails within the retry budget
- **THEN** the payment is marked `failed`, the API returns `500`, and prior saga steps (`capture`, then `authorize`) are compensated in reverse order

#### Scenario: Merchant without webhook URL skips delivery
- **WHEN** the merchant has no registered `webhook_url`
- **THEN** no HTTP call is made and creation continues as if the webhook step succeeded
