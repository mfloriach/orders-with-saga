# Design

## Context

`pkg/apiclient/apiclient.go` currently exposes `New(h *http.Client)` and a single
`Do(ctx, build func(context.Context) (*http.Request, error), Policy)` method.
Callers pass a builder closure plus a full `Policy` per call; `Policy.BaseURL`
and `Policy.APIKey` are declared but never read. See `proposal.md` for the
motivation. The behavior to preserve is in the existing `Do`: bounded attempts,
capped exponential backoff with jitter, per-attempt timeout that also wraps
request construction, draining and closing intermediate bodies, and returning
the final attempt.

## Goals / Non-Goals

**Goals:**

- A client is configured once and issues requests through verb methods.
- Retry policy and per-attempt timeout are set at construction.
- Headers configurable globally and overridable per request.
- Preserve every observable retry/backoff/timeout/cancellation behavior.

**Non-Goals:**

- Migrating callers (`internal/webhooks/consumer.go`,
  `internal/payment/repositories/mastercard.go`, `internal/app/container.go`);
  that is a follow-up (see proposal Impact).
- JSON request/response helpers — verbs still return the raw `*http.Response`.
- Streaming request bodies or connection-pool changes.

## Decisions

### Functional options over a config struct

`New(h *http.Client, opts ...Option)` with `type Option func(*config)`. Options
are `WithBaseURL`, `WithHeader`, `WithHeaders`, `WithTimeout`, `WithRetry`.
Rationale: the user asked to move timeout/retry out of a struct and to use an
option pattern; options keep the constructor readable and defaults implicit.
Alternative rejected: a `Config`/`Policy` struct — reintroduces the very struct
being removed and makes "unset" ambiguous.

### One `Option` type for construction and requests

The same `Option` applies to a `config`. `New` applies options to the client's
stored `config`; each request deep-copies it (`config.clone`, which copies the
header map) and applies its own options on top. This gives headers "global or
per request" with one API and no duplicate `RequestOption` type. Per-request
options can also override timeout/retry, which falls out for free; the design
documents this rather than forbidding it.
Alternative rejected: a separate request-option type — extra surface for no
behavioral gain.

### Verb methods over a builder closure

`Get`, `Post`, `Put`, `Patch`, `Delete` wrap a general
`Do(ctx, method, path string, body []byte, opts ...Option)`. Requests are built
internally with `http.NewRequestWithContext`. Rationale: removes the builder
closure the change targets. Bodies are `[]byte` because a retry must replay the
body: each attempt gets a fresh `bytes.NewReader`. Alternative rejected:
`io.Reader` — needs `GetBody` plumbing to retry safely.

### URL resolution

A path with an `http://`/`https://` prefix is used as-is; otherwise it is joined
to `WithBaseURL` (trimming a trailing/leading slash). Rationale: one client
shape serves both a fixed upstream base URL and posting to arbitrary absolute
targets (merchant webhook URLs). A client with no base URL passes paths through
unchanged.

### `WithRetry(maxAttempts int, baseBackoff, maxBackoff time.Duration, retryIf RetryFunc)`

Single option carrying the whole retry policy, with `RetryFunc` naming the
per-attempt predicate. A nil `retryIf` keeps the current default: retry
transport errors only and return any HTTP response as-is. Alternatives
considered: a `Retry` struct parameter, or splitting into `WithMaxAttempts` /
`WithBackoff` / `WithRetryIf` — more types or more knobs than the callers need.

### Return the raw response

Verbs return `*http.Response, error`; callers decode. Keeps the refactor
behavior-preserving. A typed `DoJSON` helper is out of scope.

## Risks / Trade-offs

- [Breaking exported API] → Follow-up change migrates the three call sites; the
  package's own tests are rewritten in this change so `go test ./pkg/apiclient`
  passes in isolation.
- [Repo does not build between this change and the caller migration] →
  Documented in proposal Impact; land the two together if a green tree is
  required.
- [Per-request options can override retry/timeout, not just headers] → Accepted;
  documented as "request options layer over client defaults".
- [Header map deep-copied per request] → Negligible for the handful of headers
  in use.

## Migration Plan

1. Rewrite `pkg/apiclient/apiclient.go` and `apiclient_test.go`; verify
   `go test ./pkg/apiclient`.
2. Follow-up change (separate) migrates the three call sites to the new API and
   restores a green build.
3. Rollback: revert the commit; no persisted state or external interface is
   affected.

## Open Questions

None. Whether to also export `Patch`/`Do` for arbitrary methods is a trivial,
deferrable detail.
