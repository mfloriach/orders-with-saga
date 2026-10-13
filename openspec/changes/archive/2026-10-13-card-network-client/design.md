# Design

## Context

See proposal.md — Why. Current state: `service.callProcessor` (`service.go:109-138`) owns the retry loop (3 attempts, 100ms doubling backoff, 10s overall) over the `Processor` seam; `container.go` injects nil. `pkg/` libraries are tiny single-package stdlib-first units (`apperr`, `saga`, `validate`). Config uses cleanenv with fail-fast validation. No HTTP client library in `go.mod` — stdlib `net/http` it is. Assumed simulator contract below is placeholder to confirm in review (test).

## Goals / Non-Goals

**Goals:**
- Reusable stdlib retry client in `pkg/`; Mastercard repo implementing the seam; service loop retires; nil-when-unconfigured preserved.
- httptest-covered retry/backoff/classification behavior.

**Non-Goals:**
- Visa, real certs/mTLS, metrics, exporter, POST-based creation flow.

## Decisions

**1. Client owns retries; service loop retires.**
`pkg/apiclient` exposes `Do(ctx, buildReq, policy)` where `Policy{MaxAttempts, BaseBackoff, MaxBackoff, Timeout, RetryIf(resp *http.Response, err error) bool}`; backoff is exponential with jitter, capped, ctx-aware between attempts. `callProcessor` becomes: single `Call` (client retries internally) → on error `Status` reconcile → in-doubt/ordinary as today. Rationale: the ask is literally "api client with retries and backoff"; two layers would multiply attempts (3x3) and stack budgets. Rejected keeping the service loop (duplication) — its stub-tested loop cases move to client httptest cases.

**2. Assumed simulator contract (confirm in review).**
Base `MC_BASE_URL`; auth `Authorization: Bearer MC_API_KEY`; JSON bodies.
- `PUT /payment/<id>` `{amount, idempotency_key}` → `200 {id, status}` — idempotent upsert; `Call` uses this (safe blind retry).
- `GET /payment/<id>` → `200 {id, status}` with status in `accepted`/`pending`/`rejected`; `Status` maps `accepted`→true, `rejected`/absent→false, anything else or transport error→in-doubt path.
- `POST /payment` with `Idempotency-Key` header → `201 {id, status}`; exposed as repo method, unused by `Create` (future key-based flows).
Rationale: PUT-by-id gives the strongest idempotency without depending on the simulator honoring key headers on retried POSTs.

**3. `Retryable` = status-code + error-kind mapping in the repo.**
Timeouts, `net.Error` temporary/timeout, `5xx` → true; `4xx` and decode failures → false. Rationale: lives with the transport knowledge; service stays policy-free.

**4. Optional vendor config.**
`MC_BASE_URL` empty → container passes nil (today's fail-closed path, `ponytail` comment stays accurate). Base URL set without `MC_API_KEY` → boot error naming the key. Rationale: local/dev boot without vendor preserved; misconfiguration still fails fast per `service-config` philosophy.

**5. Timeouts compose, never stack silently.**
Per-attempt timeout (`MC_TIMEOUT_SEC`) on the client; overall `apiTimeout` (10s, kept) in service bounds call+reconcile. Backoff sleeps respect ctx. Rationale: sync reconcile must never hold the request hostage (in-doubt trigger, per prior spec).

## Risks / Trade-offs

- [Assumed contract wrong] → mapping layer is one file; review against the real simulator before merging. Flagged in proposal assumptions.
- [Jitter complicates timing tests] → inject rand source or assert attempt counts + budget bounds, not exact sleeps.
- [POST unused] → small dead surface; kept because the user explicitly requested all three endpoints.

## Migration Plan

1. Land `pkg/apiclient` + httptest suite (no callers; inert).
2. Land repo + config + slimmed service + wiring; existing handler/payment tests cover regression.
3. Rollback = revert; nil-processor path intact when unconfigured.

## Open Questions

Contract shapes only (see decision 2) — resolve against the simulator in review; code is structured so corrections stay inside the repo file.
