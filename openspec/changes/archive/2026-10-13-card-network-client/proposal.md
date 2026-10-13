# Proposal

## Why

The `Processor` seam (`internal/payment/processor.go`) has no production implementation — `container.go` injects nil and `Create` fails closed. This change lands the first concrete client: a generic retry HTTP client in `pkg/` plus a Mastercard card-network repository implementing `Processor`, so the `call-api` saga step, retries, and sync reconcile run against a real (test/simulator) endpoint.

## What Changes

- New `pkg/apiclient`: stdlib-only HTTP client with `Do(ctx, req, policy)`; `Policy{MaxAttempts, BaseBackoff, MaxBackoff, Timeout, RetryIf}`; exponential backoff with jitter; honors ctx cancellation between attempts; no new modules.
- Retry ownership moves into the client: `service.callProcessor` slims to single `Call` + existing `Status` reconcile (its loop and backoff consts retire). One retry layer, no multiplication.
- New `internal/payment/repository_card_network_master_card.go` implementing `Processor` over the simulator:
  - `Call` = `PUT /payment/<id>` with `{amount, idempotency_key}` (idempotent create-or-confirm, safe to retry blindly).
  - `Status` = `GET /payment/<id>` mapping the status field to accepted bool.
  - `POST /payment` (with `Idempotency-Key` header) exposed as a repo method for key-based creation; not used by `Create` yet.
  - `Retryable`: timeouts, network errors, 5xx → true; 4xx/business refusal → false.
- `container.go` builds client + repo from config when `MC_BASE_URL` is set, else keeps nil (today's fail-closed dev behavior).
- New env (all optional except key-when-configured): `MC_BASE_URL`, `MC_API_KEY`, `MC_TIMEOUT_SEC=5`, `MC_MAX_ATTEMPTS=3`, `MC_BASE_BACKOFF_MS=100`.
- Out of scope: Visa/other networks, exporter/metrics for the client, real Mastercard certs (test/simulator only).

## Capabilities

### New Capabilities

None. Concrete transport behind existing payment-processing behavior.

### Modified Capabilities

- `payment-processing`: external call is now a concrete idempotent `PUT` with status-query reconcile (same observable outcomes as the seam spec).
- `service-config`: new optional `MC_*` vars with validation; absent base URL preserves current boot and fail-closed behavior.

## Impact

- New: `pkg/apiclient/`, `repository_card_network_master_card.go`, `MC_*` env.
- Modified: `service.go` (loop retires), `container.go` (wiring), `processor_test.go` (loop-count cases re-target to client tests).
- Assumptions (test; correct in review): JSON bodies; Bearer `MC_API_KEY`; status vocabulary `accepted`/`pending`/`rejected`; simulator reachable at `MC_BASE_URL`; retryable = timeout/network/5xx only.

## Assumptions

- Simulator speaks JSON over HTTP with Bearer auth; request/response shapes in design.md are placeholders to confirm against the real simulator.
- `Call` uses PUT-by-id (not POST); POST exists for future key-based flows.
- Vendor config optional: unset base URL keeps nil-processor fail-closed path.
