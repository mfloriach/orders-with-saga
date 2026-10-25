# Spec Delta

## Purpose

Lets a captured payment be handed off for asynchronous delivery through a durable outbox relay backed by the message broker.

## ADDED Requirements

### Requirement: Captured payment outbox relay
The system SHALL relay captured payments from the durable outbox to asynchronous processing through the message broker, marking the outbox row processed after a successful publish, retrying pending rows with bounded attempts, and marking them failed when attempts are exhausted.

#### Scenario: Pending captured payment is relayed
- **WHEN** the relay worker polls pending outbox rows
- **THEN** it publishes one generic captured-payment event to the broker

#### Scenario: Outbox row marked processed after successful publish
- **WHEN** the relay publish succeeds
- **THEN** the outbox row is marked processed

#### Scenario: Failed publish keeps row pending
- **WHEN** the relay publish fails
- **THEN** the outbox row keeps pending status and the attempt is recorded

#### Scenario: Exhausted attempts mark outbox row failed
- **WHEN** the relay publish attempts are exhausted
- **THEN** the outbox row is marked failed and left for operator replay
