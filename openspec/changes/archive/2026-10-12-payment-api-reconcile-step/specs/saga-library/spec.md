# Spec Delta

## ADDED Requirements

### Requirement: In-doubt skip-unwind

The system SHALL skip reverse compensation entirely when a step reports an in-doubt outcome, and return the outcome with the completed-step set intact.

#### Scenario: In-doubt skips compensation

- **WHEN** a step fails with the in-doubt signal after two steps completed
- **THEN** no compensation runs, the run returns the in-doubt outcome, and the completed steps are reported as completed

#### Scenario: Ordinary errors still unwind

- **WHEN** a step fails with any other error
- **THEN** completed steps are compensated in reverse order as before
