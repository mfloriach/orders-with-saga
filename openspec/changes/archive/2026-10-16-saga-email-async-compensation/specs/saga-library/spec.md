# Spec Delta

## MODIFIED Requirements

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
