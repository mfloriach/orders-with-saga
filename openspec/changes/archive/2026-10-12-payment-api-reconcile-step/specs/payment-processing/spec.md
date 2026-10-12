# Spec Delta

## ADDED Requirements

### Requirement: External API reconciliation

The system SHALL call the external payment API as the last creation step with retries on transient failures, and reconcile ambiguous outcomes synchronously via remote status query before responding.

#### Scenario: Clear API success

- **WHEN** the external API call succeeds within the retry budget
- **THEN** creation completes and the API returns `201` with the pending payment

#### Scenario: Clear API failure compensates

- **WHEN** the external API clearly refuses (4xx or business refusal, no retry)
- **THEN** prior steps are compensated, the payment is marked `failed`, and the API returns `500`

#### Scenario: Ambiguous outcome reconciles to success

- **WHEN** the API call outcome is ambiguous (timeout, network error, 5xx) but the status query confirms the remote accepted it
- **THEN** creation completes and the API returns `201` with the pending payment

#### Scenario: Ambiguous outcome reconciles to failure

- **WHEN** the status query confirms the remote did not accept the payment
- **THEN** prior steps are compensated, the payment is marked `failed`, and the API returns `500`

### Requirement: In-doubt outbox

The system SHALL keep the payment `pending` and record an outbox row for next-day batch processing when the external API outcome cannot be confirmed synchronously.

#### Scenario: In-doubt keeps pending and queues row

- **WHEN** retries and the status query leave the external API outcome unconfirmed
- **THEN** the payment stays `pending`, prior steps are NOT compensated, one `reconciliation_outbox` row is written with payment id and error, and the API returns `500`

#### Scenario: Client retry after in-doubt

- **WHEN** the client retries with the same `merchant_id` + idempotency key after an in-doubt response
- **THEN** the system returns `409` with the original pending `payment_id` without duplicating work
