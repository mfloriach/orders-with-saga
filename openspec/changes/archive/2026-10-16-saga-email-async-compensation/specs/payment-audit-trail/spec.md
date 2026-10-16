# Spec Delta

## Purpose

Gives operators a step-by-step record of every payment creation attempt, including why it failed, independent of the payment's terminal status.

## ADDED Requirements

### Requirement: Per-step audit records
The system SHALL write one audit row per executed saga step carrying payment id, step name, outcome, and timestamp.

#### Scenario: Successful creation records every step
- **WHEN** payment creation completes with `201`
- **THEN** one audit row exists for each executed step with outcome success

#### Scenario: Failed step records its error
- **WHEN** a saga step fails
- **THEN** that step's audit row carries outcome failure and the error text as reason

### Requirement: Failure reason recorded
The system SHALL record the failure reason on the failing step's audit row whenever payment creation does not succeed.

#### Scenario: Refusal records reason
- **WHEN** the card network clearly refuses
- **THEN** the failing step's audit row carries the refusal cause as reason

#### Scenario: In-doubt records reason
- **WHEN** the outcome cannot be confirmed synchronously
- **THEN** an audit row carries the in-doubt cause as reason alongside the outbox row
