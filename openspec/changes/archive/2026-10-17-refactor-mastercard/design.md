# Design

## Context

See proposal.md — Why. Relevant current state:

- `internal/payment/repositories/mastercard.go` does not compile: it calls
  `apiclient.Policy` and `isTransient`, both removed. It still hand-builds
  requests (`authed`) and per-call retry policy (`policy`).
- `pkg/apiclient/apiclient.go` currently retries **unconditionally** — it has no
  retry predicate, so a 4xx is retried up to `MaxAttempts`. Everything else
  (options, verbs, backoff, timeout) is already in place.
- `domain.ErrorCardNetwork{Path, Err}` exists, `Unwrap`s to its cause, and is
  currently unused.
- History: `httpError{StatusCode, Body}` plus `Retryable(err)` / `isTransient(err)`
  used to classify retryable vs terminal; both were dropped in
  `refactor: without saga`.

## Goals / Non-Goals

**Goals:**

- `mastercard.go` compiles and is a thin adapter over the configured client.
- The repo decides retryability; the client executes retries.
- All returned errors are the project's card-network error type.
- No duplicated retry/backoff logic.

**Non-Goals:**

- The service's reconcile / in-doubt / outbox flow (not implemented in
  `service.go` today; out of scope).
- Changing `apiclient`'s nil-client handling.
- Reviving the saga audit trail. The dead audit code is deleted rather than given
  a new `domain.StepRecord`, because the saga removal left it unreachable; the
  ledger table and its migration stay untouched.
- Making `NewCardNetwork` return a nil network when unconfigured (preserve
  today's behavior).

## Decisions

### 1. Client loops, repo classifies

`apiclient.Do` keeps the retry loop, backoff, jitter, and per-attempt timeout;
the repo supplies the predicate. Rationale: one retry engine, one place where
backoff lives, and the decision (transport-specific knowledge) sits with the
transport-specific repo. Alternative rejected: the repo running its own loop with
the client single-shot — it would duplicate backoff/jitter that already exist
(unexported) in `apiclient` and use none of the client's retry machinery.

### 2. Predicate on `WithRetry`

Re-introduce `type RetryFunc func(resp *http.Response, err error) bool` and make
`WithRetry(maxAttempts int, baseBackoff, maxBackoff time.Duration, retryIf RetryFunc)`
store it. `Do` evaluates it against the **raw** response/error from the send
(before any wrapping) and retries only while it returns true and attempts remain.
A nil predicate keeps today's default: retry transport errors only, return any
HTTP response as-is. Alternative rejected: a separate `WithRetryIf` option — extra
surface for no gain.

### 3. Repo classification

`repositories.Retryable(resp, err)` is a package function wired into the client at
construction:

```
  err != nil            -> retry   (transport / timeout), except context.Canceled
  resp.StatusCode >= 500 -> retry
  otherwise             -> no retry (4xx / business refusal is terminal)
```

`context.Canceled` must not trigger a retry (the caller gave up). This restores
the spec's "retry transient failures / no retry on refusal" split.

### 4. Context-only custom error

Every error the repo returns is `&domain.ErrorCardNetwork{Path: <request path>, Err: cause}`.
The cause is either the transport error from the client, a JSON marshal/unmarshal
error, or `&httpError{StatusCode, Body}` for a non-2xx. `ErrorCardNetwork.Unwrap`
exposes the cause, so callers can still `errors.As`/`errors.Is`. Alternative
rejected: a new local error type — `domain.ErrorCardNetwork` already exists for
exactly this seam and was unused.

### 5. Thin adapter, config in the client

`MasterCardRepository` holds only `*apiclient.Client`. Base URL, `Content-Type`,
`Authorization`, timeout, and retry are set via options when the client is built
in `container.go`. A single unexported `decode(resp, path, err)` closes the body
and maps the outcome to `(id, status, error)`, so all three methods share one
path. `MasterCardConfig`, `defaultOverallTimeout`, and `OverallTimeout` are
deleted.

### 6. Absolute-path resolution returns to apiclient

`Do` resolves the path through `resolve(baseURL, path)`: an `http://` or
`https://` path is used as-is, otherwise it joins the base URL with the trailing
and leading slashes trimmed. `WithBaseURL` and `Do` already document this and the
`pkg/apiclient` tests assert it; the committed code simply lost it. It is
required, not cosmetic: `webhookNotifier` posts to merchant-supplied absolute
URLs, so without it the consumer has no reachable target. The alternative — drop
the test and the documentation — would ship a client that cannot talk to
anything but its own base URL.

### 7. Residue from the saga removal

Three unrelated breaks blocked a green build and are fixed minimally, in place:

- `internal/webhooks/consumer.go` still called the removed builder-closure `Do`
  with an `apiclient.Policy`. It becomes
  `n.client.Post(ctx, target, body, WithTimeout(httpTimeout))`; the single-attempt
  intent is already the client's default, and the existing comment explains why
  the consumer owns retries.
- `internal/payment/repositories/audit.go` is deleted: `RecordStep` took the
  undefined `domain.StepRecord` and had no callers. Removing it and the `pAudit`
  argument in `container.go` also corrects `payment.NewService`, which takes 4
  parameters while the call site passed 5.
- `internal/payment/service.go` imported `"uuid"`; the module is
  `github.com/google/uuid`.

## Risks / Trade-offs

- [Predicate sees the raw error, not the wrapped one] → it is applied inside
  `Do`, before the repo wraps; the repo's `Retryable` operates on `(resp, err)`,
  never on `domain.ErrorCardNetwork`. Documented in the code.
- [Retry on a 4xx would have been silently accepted] → `Retryable` returns false
  for < 500, so the client stops after the first attempt.
- [`context.Canceled` retried] → explicitly excluded in `Retryable`.
- [Removing `MC_OVERALL_TIMEOUT_SEC`] → it is currently unused (set into a dead
  field); no runtime behavior changes.
- [Empty base URL still yields a repo] → unchanged from today; requests fail
  closed at the network step, matching the `service-config` scenario.
- [Deleting the audit code loses a ledger write path] → there is no path to lose:
  `RecordStep` had zero callers and did not compile. The table and migration
  remain for when the saga returns.
- [Restoring absolute-path resolution widens apiclient's surface] → it restores
  documented, already-tested behavior; the six failing tests are evidence it was
  expected, not new design.

## Migration Plan

1. Re-add the predicate and `resolve` to `apiclient`; verify
   `go test ./pkg/apiclient`.
2. Rewrite `mastercard.go` against the verbs; add `Retryable`; verify the package
   builds.
3. Update `container.go` wiring (card network + audit removal) and drop the dead
   env; migrate `consumer.go` and fix the `uuid` import.
4. Verify `go build ./...` and `go test ./...`.
5. Rollback: revert the commit; no persisted state or external interface changes.

## Open Questions

**The payment state machine is unimplemented and blocks a repo-wide green build.**

Two call sites in `internal/app` fail to compile, both pre-existing and outside
this change's surface:

1. `domain.PaymentRepository` declares
   `Authorized(ctx, PaymentPending, authorizationID string)`, but
   `PaymentDatabaseRepository.Authorized` takes only two arguments, so the
   repository does not satisfy the interface.
2. `payment.Service.Create` returns `*domain.PaymentCaptured`, while the
   handler's `creator` interface wants `*domain.Payment`.

Item 2 is not a signature slip. `PaymentDatabaseRepository.Captured` and
`.Failed` are stubs:

```go
func (r *PaymentDatabaseRepository) Captured(ctx context.Context, p domain.PaymentAuthenticated) (domain.PaymentCaptured, error) {
	return domain.PaymentCaptured{}, nil
}
```

They write nothing and return empty values, so `Service.Create` returns a
payment with no id and no persisted `captured` status. The flow also never
threads the authorize key: `Service.Create` calls
`s.cardNetwork.Capture(ctx, pAuthorize.ID)` — the payment id — where the card
network expects the authorize key that `Authorize` returns. The simulator's
`/payment/<authorizeId>/capture` endpoint would reject that. The old
`InsertPending` / `MarkAuthorized` / `MarkCaptured` / `MarkFailed` methods still
sit in `payments.go` as dead code next to the new names.

Making the build green therefore means implementing the `pending → authorized →
captured/failed` persistence, threading the authorization key, and deciding what
the `201` body carries (`domain.Payment` per the `payment-processing`
scenarios, which list `id`, `merchant_id`, `amount`, `status`, `created_at`).
That is a change of its own, not residue: it is behavior the current code only
pretends to have. Recommended follow-up: a new change, e.g.
`wire-payment-state-machine`, covering the repository transitions, the
authorization-key plumbing, `Captured`/`Failed`, removal of the dead
`InsertPending`/`Mark*` methods, and the handler's return type.
