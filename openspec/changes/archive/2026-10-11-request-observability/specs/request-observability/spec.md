# Spec Delta

## Purpose

Makes every request traceable across logs and responses with stable correlation IDs, without changing any endpoint behavior.

## ADDED Requirements

### Requirement: Request ID header
The system SHALL return an `X-Request-ID` header on every response, generating a unique ID when the client does not supply one.

#### Scenario: Generated request ID
- **WHEN** a client calls any endpoint without `X-Request-ID`
- **THEN** the response carries a unique `X-Request-ID` header value

#### Scenario: Client-supplied request ID honored
- **WHEN** a client calls any endpoint with `X-Request-ID: abc-123`
- **THEN** the response echoes `X-Request-ID: abc-123`

### Requirement: Request ID in error bodies
The system SHALL include `request_id` in every error response body, matching the response `X-Request-ID` value.

#### Scenario: Error carries request ID
- **WHEN** a request fails with a 4xx/5xx error body
- **THEN** the body contains `request_id` equal to the response `X-Request-ID`

### Requirement: Structured request logging
The system SHALL emit JSON logs to stdout including `request_id` for handled requests, and `trace_id` whenever a trace is active.

#### Scenario: Log correlation
- **WHEN** a request is served
- **THEN** its log lines carry the same `request_id` as the response header
