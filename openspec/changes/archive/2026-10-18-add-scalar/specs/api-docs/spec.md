# Spec Delta

## Purpose

Serves an OpenAPI definition and Scalar reference UI for the payment gateway's business endpoints so developers can discover and call the API.

## ADDED Requirements

### Requirement: OpenAPI spec served
The system SHALL expose a machine-readable OpenAPI 3.0 definition at `GET /openapi.yaml` covering the business endpoints (`POST /merchants`, `POST /payments`) with their request/response schemas and the `Idempotency-Key` header.

#### Scenario: Spec returns valid definition
- **WHEN** client GETs `/openapi.yaml`
- **THEN** system returns `200` with valid OpenAPI content describing `POST /merchants` and `POST /payments`

#### Scenario: Spec documents business endpoints
- **WHEN** client reads `/openapi.yaml`
- **THEN** it includes schemas for merchant creation, payment creation, and error responses, plus the `Idempotency-Key` request header on `POST /payments`

### Requirement: Scalar reference UI served
The system SHALL serve Scalar's API reference UI at `GET /docs`, pointing at the OpenAPI spec served by the same service.

#### Scenario: Scalar UI loads
- **WHEN** client GETs `/docs`
- **THEN** system returns `200` with Scalar HTML that loads `/openapi.yaml`

#### Scenario: Scalar UI uses served spec
- **WHEN** client GETs `/docs`
- **THEN** the Scalar UI's target spec URL is `/openapi.yaml`
