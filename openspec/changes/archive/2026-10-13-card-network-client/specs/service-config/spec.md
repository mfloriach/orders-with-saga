# Spec Delta

## ADDED Requirements

### Requirement: Card-network configuration

The system SHALL accept optional card-network settings with safe defaults, validate them at boot, and preserve current behavior when the network is unconfigured.

#### Scenario: Unconfigured network keeps fail-closed path

- **WHEN** the service starts without `MC_BASE_URL` set
- **THEN** it boots normally and payment creation fails closed with `500` at the network step, as today

#### Scenario: Invalid network values rejected

- **WHEN** the service starts with a non-positive network timeout or retry budget
- **THEN** it exits non-zero and logs an error naming the offending field, binding no port
