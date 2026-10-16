# Saga Library Specification

## Purpose

Lets service code run multi-step operations that undo completed work when a later step fails, with enough result detail for a future reconciler to resume.

## Requirements

### Requirement: Sequential saga execution

The system SHALL run saga steps in declaration order and stop at the first step failure.

#### Scenario: All steps succeed
- **WHEN** a saga with two steps runs and both steps succeed
- **THEN** the run returns success and no compensation runs

#### Scenario: Step failure stops the saga
- **WHEN** step 2 of 3 fails
- **THEN** step 3 never runs and the run returns a failure identifying step 2

#### Scenario: Cancelled context aborts forward progress
- **WHEN** the context is cancelled before a pending step starts
- **THEN** that step never runs and the run returns a failure identifying the unstarted step

### Requirement: Reverse best-effort compensation

The system SHALL compensate completed steps in reverse order when a step fails, continuing past individual compensation errors. Compensation runs in the background after the failure is returned; per-step compensation errors are reported via logs carrying saga id and step name, not aggregated into the returned error.

#### Scenario: Compensations run in reverse
- **WHEN** step 3 fails after steps 1 and 2 succeeded
- **THEN** compensation for step 2 runs before compensation for step 1

#### Scenario: Failed step has no compensation
- **WHEN** the failed step itself has a compensation defined
- **THEN** that compensation does not run; only previously completed steps are compensated

#### Scenario: Nil compensation is skipped
- **WHEN** a completed step has no compensation
- **THEN** it is skipped silently and unwinding continues

#### Scenario: Failure returns before unwind completes
- **WHEN** a step fails with an ordinary error
- **THEN** the run returns the failure promptly and completed steps are compensated asynchronously afterward

#### Scenario: Compensation errors are aggregated
- **WHEN** a compensation fails during background unwinding
- **THEN** unwinding continues for remaining steps and the error is reported via logs carrying saga id and step name instead of the returned error

### Requirement: Reconciliation-ready result

The system SHALL identify every saga run and every step by stable name and report which steps completed and which step failed with the step error. Compensation outcomes are reported via logs, not in the returned error.

#### Scenario: Result carries resume data
- **WHEN** a saga run fails
- **THEN** the error reports the saga ID, the failed step name, the step error, and the completed step names in order

#### Scenario: Saga ID defaults when unset
- **WHEN** a saga is declared without an ID
- **THEN** the run generates a unique ID and reports it in the result

### Requirement: Per-step observability

The system SHALL emit one trace span per saga and one child span per executed step and compensation, and log step failures and compensation outcomes with saga ID and step name.

#### Scenario: Step spans exist
- **WHEN** a 2-step saga runs
- **THEN** one saga span and one span per executed step are emitted, with the failed step span recording the error

#### Scenario: Failure is logged with identity
- **WHEN** a step fails or a compensation fails
- **THEN** a log record includes the saga ID, step name, and error

### Requirement: Sequential-only contract

The system SHALL execute steps sequentially and offer no parallel-branch option in this change.

#### Scenario: No parallel API
- **WHEN** a caller inspects the saga package API
- **THEN** there is no concurrency, worker-pool, or branch option to enable

### Requirement: In-doubt skip-unwind
The system SHALL skip reverse compensation entirely when a step reports an in-doubt outcome, and return the outcome with the completed-step set intact.

#### Scenario: In-doubt skips compensation
- **WHEN** a step fails with the in-doubt signal after two steps completed
- **THEN** no compensation runs, the run returns the in-doubt outcome, and the completed steps are reported as completed

#### Scenario: Ordinary errors still unwind
- **WHEN** a step fails with any other error
- **THEN** completed steps are compensated in reverse order as before
