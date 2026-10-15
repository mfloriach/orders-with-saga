# Spec Delta

## MODIFIED Requirements

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

### Requirement: Fail-fast dependency check
The system SHALL verify both Postgres pools, Redis, and RabbitMQ are reachable during boot and refuse to serve when any dependency is down.

#### Scenario: Dependency down at boot
- **WHEN** the service starts while a database, Redis, or RabbitMQ is unreachable
- **THEN** it exits non-zero, logs an error naming the dependency, and binds no port

### Requirement: Full pool shutdown
The system SHALL close the AMQP connection, Redis, and both database pools on shutdown after the server stops accepting traffic.

#### Scenario: Graceful shutdown
- **WHEN** the process receives SIGINT or SIGTERM
- **THEN** it stops accepting traffic, closes the AMQP connection, Redis and both pools, flushes telemetry, and exits
