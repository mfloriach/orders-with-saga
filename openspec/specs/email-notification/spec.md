# Email Notification Specification

## Purpose

Notifies customers asynchronously when their payment completes, without letting email delivery affect payment success or response time.

## Requirements

### Requirement: Email event published on payment success
The system SHALL publish one email event carrying payment id and merchant id when payment creation succeeds.

#### Scenario: Success publishes event
- **WHEN** payment creation completes with `201`
- **THEN** one email event for that payment is published to the broker

#### Scenario: Failed creation publishes nothing
- **WHEN** payment creation fails for any reason
- **THEN** no email event is published for that attempt

### Requirement: Async email delivery
The system SHALL deliver each email event from a background worker, independent of the payment response path, with bounded retries and dead-letter parking on exhaustion.

#### Scenario: Event delivered
- **WHEN** an email event is consumed
- **THEN** the worker sends the email and acknowledges the event

#### Scenario: Exhausted retries park in DLQ
- **WHEN** delivery failures reach the attempt bound
- **THEN** the event is moved to the dead-letter queue and a log record carries payment id and failure cause
