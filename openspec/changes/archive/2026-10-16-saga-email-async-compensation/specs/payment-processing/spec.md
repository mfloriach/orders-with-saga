# Spec Delta

## MODIFIED Requirements

### Requirement: Create payment
The system SHALL create a payment with `merchant_id` and positive integer `amount` (minor units, no currency), using the `Idempotency-Key` request header as the idempotency key and deriving one when the header is absent, with initial status `pending`. On success the system publishes webhook and email events for background delivery without affecting the response.

#### Scenario: Successful payment
- **WHEN** client POSTs `/payments` with an existing `merchant_id`, `amount > 0`, and a new `Idempotency-Key` header
- **THEN** system returns `201` with `id`, `merchant_id`, `amount`, `status=pending`, and `created_at`, echoing the key in the `Idempotency-Key` response header

#### Scenario: Invalid payment input
- **WHEN** client POSTs `/payments` with missing/non-positive `amount` or missing `merchant_id`
- **THEN** system returns `400` with code `validation_error`, a `title`, per-field `errors` details, and creates nothing

#### Scenario: Success publishes notification events
- **WHEN** payment creation completes with `201`
- **THEN** one webhook event and one email event for that payment are published, and neither delivery affects the returned response

### Requirement: External API reconciliation
The system SHALL call the external payment API as the last creation step with retries on transient failures, and reconcile ambiguous outcomes synchronously via remote status query before responding.

#### Scenario: Clear API success
- **WHEN** the external API call succeeds within the retry budget
- **THEN** creation completes and the API returns `201` with the pending payment

#### Scenario: Clear API failure compensates
- **WHEN** the external API clearly refuses (4xx or business refusal, no retry)
- **THEN** the payment is marked `failed` and the API returns `500` immediately, while prior steps are compensated in the background

#### Scenario: Ambiguous outcome reconciles to success
- **WHEN** the API call outcome is ambiguous (timeout, network error, 5xx) but the status query confirms the remote accepted it
- **THEN** creation completes and the API returns `201` with the pending payment

#### Scenario: Ambiguous outcome reconciles to failure
- **WHEN** the status query confirms the remote did not accept the payment
- **THEN** the payment is marked `failed` and the API returns `500` immediately, while prior steps are compensated in the background

### Requirement: Idempotent network authorization
The system SHALL authorize payments through an idempotent network call keyed by payment id, retrying transient failures with backoff and reconciling ambiguous outcomes via status query, preserving existing success/failed/in-doubt outcomes.

#### Scenario: Network authorizes
- **WHEN** the card-network call succeeds within the retry budget
- **THEN** creation completes and the API returns `201` with the pending payment

#### Scenario: Network refuses
- **WHEN** the card network clearly refuses (4xx or business refusal, no retry)
- **THEN** the payment is marked `failed` and the API returns `500` immediately, while prior steps are compensated in the background

#### Scenario: Ambiguous outcome reconciled
- **WHEN** the call outcome is ambiguous but the status query confirms acceptance
- **THEN** creation completes with `201`; when confirmed absent, the payment is marked `failed` with `500` while prior steps compensate in the background

#### Scenario: Unconfirmed outcome stays pending
- **WHEN** retries and status query leave the outcome unconfirmed
- **THEN** the payment stays `pending`, one `reconciliation_outbox` row is written, and the API returns `500`
