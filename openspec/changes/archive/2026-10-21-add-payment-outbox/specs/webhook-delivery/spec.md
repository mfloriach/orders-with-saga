# Spec Delta

## MODIFIED Requirements

### Requirement: Synchronous webhook call within payment creation
The system SHALL publish one webhook event to the outbox table as part of payment creation without blocking the response.

#### Scenario: Webhook call succeeds
- **WHEN** payment creation completes with `201` and the merchant has a `webhook_url`
- **THEN** one outbox row exists with the payment id, event type payment.succeeded, and the webhook payload; the API returns `201`

#### Scenario: Webhook call fails compensates prior steps
- **WHEN** an outbox webhook event cannot be written
- **THEN** the payment remains `captured` and the API returns `201`; the failure does not trigger compensation

#### Scenario: Merchant without webhook URL skips delivery
- **WHEN** the merchant has no registered `webhook_url`
- **THEN** no outbox row is created and creation continues as if the webhook step succeeded

## ADDED Requirements

### Requirement: Webhook delivery failure does not fail the payment
The system SHALL leave the payment in `captured` status when a webhook outbox event cannot be processed, so delivery failures never mark a captured payment as failed.

#### Scenario: Undeliverable webhook keeps payment captured
- **WHEN** an outbox webhook event cannot be delivered to the merchant endpoint
- **THEN** the payment remains `captured` and the API returns `201`
