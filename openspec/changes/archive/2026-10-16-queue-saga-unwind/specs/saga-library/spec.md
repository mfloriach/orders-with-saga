# Spec Delta

## MODIFIED Requirements

### Requirement: Reverse best-effort compensation
The system SHALL compensate completed steps in reverse order when a step fails, continuing past individual compensation errors. The run records a durable compensation intent and returns the failure promptly; a worker executes the queued unwind, and per-step compensation errors are reported via worker logs carrying saga id and step name, not aggregated into the returned error.

#### Scenario: Compensations run in reverse
- **WHEN** step 3 fails after steps 1 and 2 succeeded
- **THEN** the recorded intent orders step 2 before step 1 and the worker compensates in that order

#### Scenario: Failed step has no compensation
- **WHEN** the failed step itself has a compensation defined
- **THEN** that compensation does not run; only previously completed steps are compensated

#### Scenario: Nil compensation is skipped
- **WHEN** a completed step has no compensation
- **THEN** it is skipped silently and unwinding continues

#### Scenario: Failure returns before unwind completes
- **WHEN** a step fails with an ordinary error
- **THEN** the run returns the failure promptly and the queued unwind runs afterward in the worker

#### Scenario: Compensation errors are aggregated
- **WHEN** a compensation fails during worker unwinding
- **THEN** unwinding continues for remaining steps and the error is reported via worker logs carrying saga id and step name instead of the returned error
