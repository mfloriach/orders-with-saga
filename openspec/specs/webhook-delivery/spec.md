# Webhook Delivery Specification

## Purpose

Notifies merchants asynchronously when their payments complete, with bounded retries and dead-letter parking so an unreliable merchant endpoint never blocks payment responses.

## Requirements

### Requirement: Webhook event published on payment success
The system SHALL publish one webhook event carrying payment id, merchant id, amount, and status when payment creation succeeds.

#### Scenario: Success publishes event
- **WHEN** payment creation completes with `201`
- **THEN** one webhook event for that payment is published to the broker

#### Scenario: Failed creation publishes nothing
- **WHEN** payment creation fails (validation, refusal, or in-doubt outcome)
- **THEN** no webhook event is published for that attempt

### Requirement: Async webhook delivery to merchant endpoint
The system SHALL deliver each webhook event to the merchant's registered `webhook_url` from a background worker, independent of the payment response path.

#### Scenario: Event delivered
- **WHEN** a webhook event is consumed and the merchant has a `webhook_url`
- **THEN** the worker POSTs the payment payload to that URL

#### Scenario: Merchant without webhook URL skips delivery
- **WHEN** a webhook event is consumed for a merchant with no `webhook_url`
- **THEN** the event is acknowledged with no HTTP call and no retry

### Requirement: Bounded retries with dead-letter parking
The system SHALL retry failed webhook deliveries with backoff up to a bounded attempt count, then park the event in a dead-letter queue and surface it in logs.

#### Scenario: Transient failure retries
- **WHEN** delivery fails with a transient error below the attempt bound
- **THEN** the event is requeued with backoff and attempted again

#### Scenario: Exhausted retries park in DLQ
- **WHEN** delivery failures reach the attempt bound
- **THEN** the event is moved to the dead-letter queue, acknowledged from the main queue, and a log record carries payment id and failure cause
