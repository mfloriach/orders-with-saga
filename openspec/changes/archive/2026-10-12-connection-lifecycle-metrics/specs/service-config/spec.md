# Spec Delta

## ADDED Requirements

### Requirement: Pool and timeout configuration

The system SHALL accept pool and timeout tuning via environment with safe defaults and reject invalid values at boot with a naming error.

#### Scenario: Defaults boot unchanged

- **WHEN** the service starts with only the three required DSNs set
- **THEN** it boots with default pool sizes and timeouts and serves traffic normally

#### Scenario: Invalid pool value rejected

- **WHEN** the service starts with a non-positive pool size or a negative timeout
- **THEN** it exits non-zero and logs an error naming the offending field, binding no port

### Requirement: Fail-fast dependency check

The system SHALL verify both Postgres pools and Redis are reachable during boot and refuse to serve when any dependency is down.

#### Scenario: Dependency down at boot

- **WHEN** the service starts while a database or Redis is unreachable
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

The system SHALL close Redis and both database pools on shutdown after the server stops accepting traffic.

#### Scenario: Graceful shutdown

- **WHEN** the process receives SIGINT or SIGTERM
- **THEN** it stops accepting traffic, closes Redis and both pools, flushes telemetry, and exits
