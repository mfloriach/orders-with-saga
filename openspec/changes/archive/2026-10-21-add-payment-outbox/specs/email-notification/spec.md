# Spec Delta

## MODIFIED Requirements

### Requirement: Email event published on payment success
The system SHALL publish one email event to the outbox table carrying payment id and merchant id when payment creation succeeds, without blocking the response.

#### Scenario: Success publishes event
- **WHEN** payment creation completes with `201`
- **THEN** one outbox row exists with the payment id, event type payment.succeeded, and the email payload; the API returns `201`

#### Scenario: Failed creation publishes nothing
- **WHEN** payment creation fails for any reason
- **THEN** no outbox row is created for that attempt

## ADDED Requirements

### Requirement: Email delivery failure does not fail the payment
The system SHALL leave the payment in `captured` status when an email outbox event cannot be processed, so delivery failures never mark a captured payment as failed.

#### Scenario: Undeliverable email keeps payment captured
- **WHEN** an outbox email event cannot be delivered to the customer
- **THEN** the payment remains `captured` and the API returns `201`
