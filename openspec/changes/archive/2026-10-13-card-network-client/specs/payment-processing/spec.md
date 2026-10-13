# Spec Delta

## ADDED Requirements

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
