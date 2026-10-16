# Spec Delta

## Purpose

Makes saga compensation durable by queueing the unwind instead of running it in a fire-and-forget goroutine, so no deploy, crash, or restart can silently drop owed compensation.

## ADDED Requirements

### Requirement: Compensation intent recorded on failure
The system SHALL record one durable compensation intent carrying saga id, saga name, failed step, step error, and the completed step names in order whenever a saga run fails with an ordinary error.

#### Scenario: Failure records intent
- **WHEN** a saga step fails with an ordinary error
- **THEN** one compensation intent exists carrying the saga id, the failed step name, and the completed step names in order

#### Scenario: In-doubt records no compensation intent
- **WHEN** a saga step fails with the in-doubt signal
- **THEN** no compensation intent is recorded and the outcome is handled by the reconciliation path

#### Scenario: Failure returns before unwind completes
- **WHEN** a step fails with an ordinary error
- **THEN** the run returns the failure promptly and the recorded intent is queued for the worker

### Requirement: Worker executes queued unwind
The system SHALL execute each queued compensation intent in a worker by running compensations for the recorded completed steps in reverse order, continuing past individual compensation errors, and reporting them via logs carrying saga id and step name.

#### Scenario: Compensations run in reverse
- **WHEN** a worker consumes an intent whose completed steps are step 1 then step 2
- **THEN** compensation for step 2 runs before compensation for step 1

#### Scenario: Failed step has no compensation
- **WHEN** the failed step itself has a compensation defined
- **THEN** that compensation does not run; only previously completed steps are compensated

#### Scenario: Nil compensation is skipped
- **WHEN** a completed step has no compensation
- **THEN** it is skipped silently and unwinding continues

#### Scenario: Intent acknowledged after full unwind
- **WHEN** every recorded compensation has run
- **THEN** the intent is acknowledged and the payment is left in its terminal state

### Requirement: Idempotent compensation replay
The system SHALL treat compensation replay as a no-op for already-applied effects, so redelivered intents never produce a second side effect.

#### Scenario: Redelivered intent replays safely
- **WHEN** an intent is consumed a second time after a crash between unwind and acknowledge
- **THEN** already-applied compensations apply nothing further and the intent is acknowledged

### Requirement: Exhausted unwind parks in DLQ
The system SHALL move a compensation intent to the dead-letter queue with payment id and failure cause when delivery failures reach the attempt bound, leaving the intent record for sweeper replay.

#### Scenario: Exhausted retries park in DLQ
- **WHEN** compensation delivery failures reach the attempt bound
- **THEN** the message is moved to the dead-letter queue and a log record carries saga id and failure cause

#### Scenario: Unknown step dead-letters immediately
- **WHEN** an intent names a step outside the worker's registry
- **THEN** the message is dead-lettered without retry and the intent record is kept

#### Scenario: Parked intent remains replayable
- **WHEN** a compensation intent sits in the dead-letter queue
- **THEN** its intent record still exists and a replay re-queues it without duplicating applied effects

### Requirement: No new payment status while unwind is queued
The system SHALL leave the payment row in its last step state while compensation is queued, recording unwind-owed state only in the intent record.

#### Scenario: Row keeps last step state
- **WHEN** a saga fails and its unwind is queued but not yet executed
- **THEN** the payment row still carries the state written by the last executed step
