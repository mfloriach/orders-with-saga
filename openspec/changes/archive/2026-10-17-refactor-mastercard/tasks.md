# Tasks

## 1. apiclient: restore the retry predicate and URL resolution

- [x] 1.1 Re-add `type RetryFunc func(resp *http.Response, err error) bool`; add a
  `retryIf RetryFunc` field to the config and make `WithRetry` take it as a fourth
  argument.
- [x] 1.2 In `Do`, evaluate the predicate against the raw response/error and retry
  only while it returns true and attempts remain. Nil predicate = retry transport
  errors only.
- [x] 1.3 Add a false-predicate case to `pkg/apiclient` (the tests already used the
  4-argument `WithRetry` signature).
- [x] 1.4 Restore absolute-path resolution: an `http(s)://` path bypasses the base
  URL, otherwise join with slashes trimmed. This closes the gap the committed
  `refactor-apiclient` left behind; `go test ./pkg/apiclient` is green afterwards.

## 2. Repo rewrite

- [x] 2.1 Delete `MasterCardConfig`, `defaultOverallTimeout`, the `OverallTimeout`
  field, and the `authed` / `policy` builders.
- [x] 2.2 Reduce `MasterCardRepository` to `*apiclient.Client` with
  `NewMasterCardRepository(client *apiclient.Client)`.
- [x] 2.3 Add `Retryable(resp, err)`: transport/timeout errors retry (except
  `context.Canceled`), 5xx retries, everything else does not.
- [x] 2.4 Add `httpError{StatusCode, Body}` and a shared `decode(resp, path, err)`
  returning `(id, status, error)`, wrapping every failure in
  `&domain.ErrorCardNetwork{Path, Err}`.
- [x] 2.5 Rewrite `Authorize` / `Capture` / `GetPayment` on the client verbs; fix
  `Authorize` to return the decoded authorize id.
- [x] 2.6 Remove the `isTransient` reference and confirm the package compiles
  (`go build ./internal/payment/repositories`).

## 3. Wiring and config

- [x] 3.1 Update `container.go` `NewCardNetwork` to build the client with base URL,
  `Content-Type`, conditional `Authorization`, timeout, and
  `WithRetry(..., repositories.Retryable)`, then inject it into the repo.
- [x] 3.2 Remove the now-unused `MC_OVERALL_TIMEOUT_SEC` env and its wiring.

## 4. Residue from the saga removal (approved scope extension)

- [x] 4.1 Delete the dead audit code: `internal/payment/repositories/audit.go`
  (`RecordStep` referenced the undefined `domain.StepRecord` and had no callers)
  and drop the `pAudit` wiring in `container.go`, which also repairs the
  5-args-vs-4-params `payment.NewService` call.
- [x] 4.2 Fix `internal/payment/service.go`'s `"uuid"` import to
  `github.com/google/uuid`.
- [x] 4.3 Migrate `internal/webhooks/consumer.go` to the verb API
  (`n.client.Post(..., WithTimeout(httpTimeout))`), dropping the removed builder
  closure and `apiclient.Policy`.

## 5. Verify

- [ ] 5.1 `go build ./...` and `go test ./...` are green.

  Blocked by two pre-existing mismatches outside this change's surface, both in
  `internal/app`'s call sites:
  `PaymentDatabaseRepository.Authorized` is missing the `authorizationID`
  parameter the interface now declares, and `payment.Service.Create` returns
  `*domain.PaymentCaptured` while the handler's `creator` interface expects
  `*domain.Payment`. The second is not merely a type mismatch —
  `PaymentDatabaseRepository.Captured` and `.Failed` are stubs that persist
  nothing and return empty values, so satisfying it means implementing the
  payment state machine, not adjusting a signature. See design.md, Open
  Questions.

- [x] 5.2 Sanity-check the seam: a 4xx response is not retried; an
  `&domain.ErrorCardNetwork{}` is returned with an inspectable cause.
  (`internal/payment/repositories/mastercard_test.go`.)
