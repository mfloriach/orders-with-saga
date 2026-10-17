# Proposal

## Why

`internal/payment/repositories/mastercard.go` is stuck between two `apiclient`
generations and no longer compiles: it calls the removed `apiclient.Policy` and
the deleted `isTransient`, and hand-builds requests and retry config that the
redesigned client now owns. It also silently drops the authorize key
(`Authorize` always returns `""`).

## What Changes

- **BREAKING** `pkg/apiclient`: `WithRetry` regains a per-attempt predicate —
  `WithRetry(maxAttempts int, baseBackoff, maxBackoff time.Duration, retryIf RetryFunc)`.
  `Do` retries only while the predicate says so; a nil predicate keeps today's
  default of retrying transport errors only. (`RetryFunc` returns.)
- Rewrite `MasterCardRepository` as a thin adapter over the configured
  `apiclient.Client`: `Authorize`, `Capture`, and `GetPayment` call the client's
  verbs directly. The `authed`, `policy`, and request-builder closures are removed.
- **BREAKING** Drop `MasterCardConfig` and the
  `NewMasterCardRepository(client, cfg)` signature; the client now carries base
  URL, auth header, timeout, and retry. New signature:
  `NewMasterCardRepository(client *apiclient.Client)`.
- Retry classification lives in the repo as `Retryable(resp, err)`: transport /
  timeout errors and 5xx retry; 4xx and everything else do not.
- Every returned error is wrapped in the existing
  `domain.ErrorCardNetwork{Path, Err}` (context-only custom error). A non-2xx
  cause is the local `httpError{StatusCode, Body}`; `Unwrap` keeps the cause
  inspectable.
- Fix `Authorize` to return the decoded authorize id instead of always `""`.
- Remove the dead `OverallTimeout` field, `defaultOverallTimeout`, and the
  now-unused `MC_OVERALL_TIMEOUT_SEC` env.
- Update `container.go` `NewCardNetwork` to build the configured client (base
  URL, headers, timeout, retry) and inject the repo.
- Restore absolute-path resolution in `pkg/apiclient`: the committed
  `refactor-apiclient` dropped it while its own tests and docs still require it
  (six tests fail at `HEAD`). One client must serve a fixed upstream base URL and
  arbitrary absolute targets, which is exactly how the webhook consumer works.
- Migrate `internal/webhooks/consumer.go` to the verb API. It still used the
  removed builder closure and `apiclient.Policy`, so `internal/webhooks` did not
  compile.
- Delete the dead audit code (`internal/payment/repositories/audit.go`), whose
  `RecordStep` referenced the undefined `domain.StepRecord` and had no callers
  since the saga removal, along with its `pAudit` wiring. That also repairs
  `container.go`'s 5-argument call to `payment.NewService`, which takes 4.
- Fix `internal/payment/service.go`'s `"uuid"` import (the module is
  `github.com/google/uuid`).

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- None. This is a refactor plus conformance to existing `payment-processing`
  behavior (retry transient failures, no retry on a clear refusal); no
  requirement text changes. `skip_specs: true` is set in `.openspec.yaml`.

## Impact

- Code: `pkg/apiclient/apiclient.go` (and its test),
  `internal/payment/repositories/mastercard.go`,
  `internal/payment/repositories/audit.go` (deleted),
  `internal/payment/service.go`, `internal/app/container.go`,
  `internal/config/config.go`, `internal/webhooks/consumer.go`.
- Dependencies: none added; stdlib only.
- The `MC_OVERALL_TIMEOUT_SEC` removal changes `service-config` wiring but not
  its requirements (the var is currently unused).
- Scope note: the last four bullets are pre-existing breakage the repo carried
  from the saga removal and the apiclient refactor. They are included because the
  build and test suite cannot go green otherwise; each fix is minimal and no
  behavior is added beyond restoring what the docs and tests already promised.
