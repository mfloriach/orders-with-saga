# Spec Delta

## ADDED Requirements

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
