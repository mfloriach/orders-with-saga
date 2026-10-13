# Payment Processing Specification

## Purpose

Lets merchants accept payments exactly once, with a durable ledger record and protection against retries and abuse.

## Requirements

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

### Requirement: Ledger recording
The system SHALL persist one ledger entry per created payment in the ledger database.

#### Scenario: Ledger entry written
- **WHEN** a payment is created
- **THEN** the ledger database contains one entry with that `payment_id` and matching `amount`

#### Scenario: Ledger write failure
- **WHEN** the ledger write fails after the payment row was inserted
- **THEN** the payment is marked `failed` and the API returns `500` instead of a false success

### Requirement: Rate limiting
The system SHALL rate-limit payment and merchant endpoints per client IP using Redis.

#### Scenario: Rate limit exceeded
- **WHEN** a client exceeds 100 requests/minute per IP
- **THEN** system returns `429` with a `retry_after` hint, a `title`, and `details`

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

### Requirement: Idempotent network authorization
The system SHALL authorize payments through an idempotent network call keyed by payment id, retrying transient failures with backoff and reconciling ambiguous outcomes via status query, preserving existing success/failed/in-doubt outcomes.

#### Scenario: Network authorizes
- **WHEN** the card-network call succeeds within the retry budget
- **THEN** creation completes and the API returns `201` with the pending payment

#### Scenario: Network refuses
- **WHEN** the card network clearly refuses (4xx or business refusal, no retry)
- **THEN** prior steps are compensated, the payment is marked `failed`, and the API returns `500`

#### Scenario: Ambiguous outcome reconciled
- **WHEN** the call outcome is ambiguous but the status query confirms acceptance
- **THEN** creation completes with `201`; when confirmed absent, prior steps compensate and the API returns `500`

#### Scenario: Unconfirmed outcome stays pending
- **WHEN** retries and status query leave the outcome unconfirmed
- **THEN** the payment stays `pending`, one `reconciliation_outbox` row is written, and the API returns `500`

### Requirement: Authorized payment state
The system SHALL mark a payment `authorized` when the card network confirms funds held, distinguishing it from unprocessed `pending` and terminal `failed`.

#### Scenario: Network accept marks authorized
- **WHEN** the card-network call or status-query reconcile confirms acceptance
- **THEN** the payment is marked `authorized` and the API returns `201` with `status=authorized`

#### Scenario: In-doubt stays pending
- **WHEN** the network outcome cannot be confirmed
- **THEN** the payment stays `pending` (not `authorized`), prior steps are NOT compensated, and one `reconciliation_outbox` row is written

#### Scenario: Batch distinguishes accepted from orphaned
- **WHEN** a later process lists payments needing attention
- **THEN** `authorized` payments are excluded without joining the outbox, while `pending` rows remain candidates for status-query resolution

### Requirement: Payment state transitions
The system SHALL allow only defined status transitions and reject all others before touching the database.

#### Scenario: Legal transitions succeed
- **WHEN** a payment moves `pending→authorized`, `pending→failed`, `authorized→failed`, or `authorized→captured`
- **THEN** the transition is applied

#### Scenario: Illegal transition rejected
- **WHEN** code attempts any other transition (e.g. `failed→authorized`, `captured→pending`, or from an unknown state)
- **THEN** the operation fails fast with an error and no database write occurs
