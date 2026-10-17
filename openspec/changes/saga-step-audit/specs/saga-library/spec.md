# Spec Delta

## MODIFIED Requirements

### Requirement: Per-step observability
The system SHALL emit one trace span per saga and one child span per executed step and compensation, and log step failures and compensation outcomes with saga ID and step name. The saga additionally invokes a synchronous per-step observer after each executed step with the step name and result; the observer cannot fail the saga.

#### Scenario: Step spans exist
- **WHEN** a 2-step saga runs
- **THEN** one saga span and one span per executed step are emitted, with the failed step span recording the error

#### Scenario: Failure is logged with identity
- **WHEN** a step fails or a compensation fails
- **THEN** a log record includes the saga ID, step name, and error

#### Scenario: Observer sees every executed step
- **WHEN** a saga runs steps including a failed one
- **THEN** the observer is invoked once per executed step with its outcome, and a step never executed is never reported
