# Service Config Specification

## Purpose

Ensures the service boots only with valid configuration, failing fast with a clear error instead of serving traffic with wrong defaults.

## Requirements

### Requirement: Required connection vars enforced
The system SHALL refuse to boot when `APP_DSN`, `LEDGER_DSN`, `REDIS_ADDR`, or `RABBITMQ_URL` is missing or empty, exiting non-zero before binding any port.

#### Scenario: Missing app DSN
- **WHEN** the service starts without `APP_DSN` set
- **THEN** it exits non-zero, logs an error naming `AppDSN`, and binds no port

#### Scenario: Typo in var name
- **WHEN** the operator sets `APP_DNS` (typo) while `APP_DSN` is absent
- **THEN** startup fails naming `AppDSN` instead of silently using a localhost default

#### Scenario: Missing RabbitMQ URL
- **WHEN** the service starts without `RABBITMQ_URL` set
- **THEN** it exits non-zero, logs an error naming the RabbitMQ URL field, and binds no port

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

### Requirement: Pool and timeout configuration
The system SHALL accept pool and timeout tuning via environment with safe defaults and reject invalid values at boot with a naming error.

#### Scenario: Defaults boot unchanged
- **WHEN** the service starts with only the three required DSNs set
- **THEN** it boots with default pool sizes and timeouts and serves traffic normally

#### Scenario: Invalid pool value rejected
- **WHEN** the service starts with a non-positive pool size or a negative timeout
- **THEN** it exits non-zero and logs an error naming the offending field, binding no port

### Requirement: Fail-fast dependency check
The system SHALL verify both Postgres pools, Redis, and RabbitMQ are reachable during boot and refuse to serve when any dependency is down.

#### Scenario: Dependency down at boot
- **WHEN** the service starts while a database, Redis, or RabbitMQ is unreachable
- **THEN** it exits non-zero, logs an error naming the dependency, and binds no port

### Requirement: Readiness endpoint
The system SHALL expose `GET /ready` reporting per-dependency reachability while `GET /health` keeps its existing liveness response unchanged.

#### Scenario: All dependencies reachable
- **WHEN** a client calls `GET /ready` with all dependencies up
- **THEN** it returns `200` with per-dependency ok status

#### Scenario: Dependency down at runtime
- **WHEN** a client calls `GET /ready` while a dependency is unreachable
- **THEN** it returns `503` naming the failing dependency, while `GET /health` still returns `200`

### Requirement: Full pool shutdown
The system SHALL close the AMQP connection, Redis, and both database pools on shutdown after the server stops accepting traffic.

#### Scenario: Graceful shutdown
- **WHEN** the process receives SIGINT or SIGTERM
- **THEN** it stops accepting traffic, closes the AMQP connection, Redis and both pools, flushes telemetry, and exits

### Requirement: Card-network configuration
The system SHALL accept optional card-network settings with safe defaults, validate them at boot, and preserve current behavior when the network is unconfigured.

#### Scenario: Unconfigured network keeps fail-closed path
- **WHEN** the service starts without `MC_BASE_URL` set
- **THEN** it boots normally and payment creation fails closed with `500` at the network step, as today

#### Scenario: Invalid network values rejected
- **WHEN** the service starts with a non-positive network timeout or retry budget
- **THEN** it exits non-zero and logs an error naming the offending field, binding no port

### Requirement: SMTP configuration for email sending
The system SHALL accept SMTP configuration via environment variables with safe defaults, validate them at boot, and reject invalid values with a naming error.

#### Scenario: Minimal valid SMTP config
- **WHEN** the service starts with `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS` set to valid values
- **THEN** it boots and email sending is available

#### Scenario: Missing SMTP host
- **WHEN** the service starts without `SMTP_HOST` set
- **THEN** it boots normally but email sending is disabled (no-op), logging a warning

#### Scenario: Invalid SMTP port
- **WHEN** the service starts with non-numeric or out-of-range `SMTP_PORT`
- **THEN** it exits non-zero and logs an error naming `SMTPPort`

#### Scenario: Invalid SMTP TLS setting
- **WHEN** the service starts with invalid `SMTP_TLS` value (not "none", "starttls", or "tls")
- **THEN** it exits non-zero and logs an error naming `SMTPTLS`

#### Scenario: Defaults preserved when unset
- **WHEN** the service starts with only `SMTP_HOST` and `SMTP_PORT` set
- **THEN** it boots with default `SMTP_TLS=starttls`, `SMTP_FROM="noreply@localhost"`, and email sending works
