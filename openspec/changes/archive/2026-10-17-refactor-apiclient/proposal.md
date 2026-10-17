# Proposal

## Why

`pkg/apiclient` is built around a per-call seam: `Do(ctx, build, Policy)` forces
every caller to pass a request-builder closure and a full `Policy`, while the
`Policy` struct's `BaseURL` and `APIKey` fields are dead (never read by the
client). Request construction and retry wiring thus live at every call site
instead of in the client that owns them.

## What Changes

- **BREAKING** Replace `New(h *http.Client)` plus a per-call `Policy` with a
  functional-options constructor: `New(h *http.Client, opts ...Option)`.
- **BREAKING** Move retry configuration into construction via
  `WithRetry(maxAttempts, baseBackoff, maxBackoff, retryIf)`; move the
  per-attempt timeout to `WithTimeout(d)`.
- **BREAKING** Add verb methods `Get`, `Post`, `Put`, `Patch`, `Delete` (over a
  general `Do`) that build the `*http.Request` internally. The
  `build func(context.Context) (*http.Request, error)` parameter is removed.
- Headers configurable globally (`WithHeader` / `WithHeaders` at construction)
  and per request (the same options, applied on top of the client's defaults,
  with per-request values winning).
- Base URL via `WithBaseURL`; an absolute request path bypasses it.
- Request bodies are `[]byte` so each retry attempt can replay them.
- Remove the `Policy` struct and its dead `BaseURL` / `APIKey` fields.
- Rewrite `apiclient_test.go` against the new API, preserving the existing
  retry, backoff, timeout, cancellation, and body round-trip behavior.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- None. This is a pure refactor: observable behavior (bounded retries,
  jittered capped backoff, per-attempt timeout, draining and closing
  intermediate response bodies, returning the final attempt) is unchanged, and
  no existing spec requirement changes. `skip_specs: true` is set in
  `.openspec.yaml`.

## Impact

- Code: `pkg/apiclient/apiclient.go`, `pkg/apiclient/apiclient_test.go`.
- Out of scope for this change: migrating call sites
  (`internal/webhooks/consumer.go`, `internal/payment/repositories/mastercard.go`,
  `internal/app/container.go`). The exported-API change breaks them, so they
  must be migrated in a follow-up change to keep the build green.
- Dependencies: none added; stdlib only.
