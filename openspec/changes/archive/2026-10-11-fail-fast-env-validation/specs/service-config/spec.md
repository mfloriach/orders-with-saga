# Spec Delta

## Purpose

Ensures the service boots only with valid configuration, failing fast with a clear error instead of serving traffic with wrong defaults.

## ADDED Requirements

### Requirement: Required connection vars enforced
The system SHALL refuse to boot when `APP_DSN`, `LEDGER_DSN`, or `REDIS_ADDR` is missing or empty, exiting non-zero before binding any port.

#### Scenario: Missing app DSN
- **WHEN** the service starts without `APP_DSN` set
- **THEN** it exits non-zero, logs an error naming `AppDSN`, and binds no port

#### Scenario: Typo in var name
- **WHEN** the operator sets `APP_DNS` (typo) while `APP_DSN` is absent
- **THEN** startup fails naming `AppDSN` instead of silently using a localhost default

### Requirement: Invalid values rejected
The system SHALL refuse to boot when `PORT` is non-numeric or out of range, or when rate-limit values are not positive integers.

#### Scenario: Zero rate limit
- **WHEN** the service starts with `RATE_LIMIT=0`
- **THEN** it exits non-zero and logs an error naming `RateLimit`

#### Scenario: Bad port
- **WHEN** the service starts with `PORT=abc`
- **THEN** it exits non-zero and logs an error naming `PORT`

### Requirement: Safe defaults preserved
The system SHALL boot with defaults `PORT=8080`, `RATE_LIMIT=100`, `RATE_WINDOW_SEC=60` when those vars are unset.

#### Scenario: Minimal valid env
- **WHEN** the service starts with only the three required DSNs set
- **THEN** it boots and serves `GET /health` normally
