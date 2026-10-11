# Merchant Registration Specification

## Purpose

Lets the platform onboard merchants so payments can be attributed to a known merchant account.

## Requirements

### Requirement: Register merchant
The system SHALL register a merchant with name, email, and optional webhook URL, assigning a unique merchant ID.

#### Scenario: Successful registration
- **WHEN** client POSTs `/merchants` with valid `name`, `email`, and optional `webhook_url`
- **THEN** system returns `201` with `id`, `name`, `email`, `webhook_url`, and `created_at`

#### Scenario: Validation failure
- **WHEN** client POSTs `/merchants` with missing/invalid `name` or `email`, or non-HTTP(S) `webhook_url`
- **THEN** system returns `400` with per-field error details and creates nothing

### Requirement: Merchant email uniqueness
The system SHALL reject a merchant registration when `email` is already registered.

#### Scenario: Duplicate email
- **WHEN** client POSTs `/merchants` with an `email` already in use
- **THEN** system returns `409` with an `email_taken` error code

### Requirement: Lookup merchant for payment attribution
The system SHALL resolve a merchant by ID when creating a payment.

#### Scenario: Unknown merchant on payment
- **WHEN** client POSTs `/payments` with a `merchant_id` that does not exist
- **THEN** system returns `404` with a `merchant_not_found` error code
