# Spec Delta

## Purpose

Provides a durable outbox table for payment-side-effect events so webhook and email delivery can happen asynchronously without blocking payment creation responses.

## ADDED Requirements

### Requirement: Outbox event row written with payment state change
The system SHALL write one outbox event row carrying the payment id, event type, and payload in the same database transaction as the payment state change it accompanies.

#### Scenario: Outbox row created with captured payment
- **WHEN** a payment transitions to captured and a side-effect event is due
- **THEN** one outbox row exists with the payment id, the event type, and the event payload, written in the same transaction as the capture

### Requirement: Processed outbox rows are deleted
The system SHALL delete an outbox row after its event is successfully processed.

#### Scenario: Row removed after processing
- **WHEN** an outbox event has been successfully processed
- **THEN** the corresponding outbox row is deleted from the table
