# Email Notification Specification

## Purpose

Notifies customers asynchronously when their payment completes, without letting email delivery affect payment success or response time.

## Requirements

### Requirement: Email event published on payment success
The system SHALL publish one email event to the outbox table carrying payment id and merchant id when payment creation succeeds, without blocking the response.

#### Scenario: Success publishes event
- **WHEN** payment creation completes with `201`
- **THEN** one outbox row exists with the payment id, event type payment.succeeded, and the email payload; the API returns `201`

#### Scenario: Failed creation publishes nothing
- **WHEN** payment creation fails for any reason
- **THEN** no outbox row is created for that attempt

### Requirement: Email delivery failure does not fail the payment
The system SHALL leave the payment in `captured` status when an email outbox event cannot be processed, so delivery failures never mark a captured payment as failed.

#### Scenario: Undeliverable email keeps payment captured
- **WHEN** an outbox email event cannot be delivered to the customer
- **THEN** the payment remains `captured` and the API returns `201`

### Requirement: Async email delivery
The system SHALL deliver each email event from a background worker, independent of the payment response path, with bounded retries and dead-letter parking on exhaustion.

#### Scenario: Event delivered
- **WHEN** an email event is consumed
- **THEN** the worker sends the email and acknowledges the event

#### Scenario: Exhausted retries park in DLQ
- **WHEN** delivery failures reach the attempt bound
- **THEN** the event is moved to the dead-letter queue and a log record carries payment id and failure cause

### Requirement: Refund email sent on refund initiation
The system SHALL send a refund-in-progress email to the merchant carrying the refund id, amount, merchant name, and refund reason when a refund is initiated.

#### Scenario: Refund email is sent
- **WHEN** a refund is initiated for a captured payment
- **THEN** one refund-in-progress email is sent to the merchant

#### Scenario: No refund email when initiation fails
- **WHEN** refund initiation fails
- **THEN** no refund-in-progress email is sent for that attempt
