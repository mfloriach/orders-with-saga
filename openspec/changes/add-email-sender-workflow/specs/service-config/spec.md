# Spec Delta

## ADDED Requirements

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