# Tasks

## 1. Options and client construction

- [x] 1.1 Define `Option func(*config)`, `RetryFunc`, and the unexported `config` fields (`baseURL`, `headers`, `maxAttempts`, `baseBackoff`, `maxBackoff`, `timeout`, `retryIf`); verify `go build ./pkg/apiclient` compiles.
- [x] 1.2 Implement `WithBaseURL`, `WithHeader`, `WithHeaders`, `WithTimeout`, `WithRetry(maxAttempts int, baseBackoff, maxBackoff time.Duration, retryIf RetryFunc)`; verify with a table-driven test asserting each option writes its field.
- [x] 1.3 Add `config.clone` (deep-copies the header map) and `config.resolve` (absolute `http(s)://` path passes through, otherwise join to `baseURL`, trimming slashes); verify with unit tests for both.
- [x] 1.4 Replace `New(h *http.Client)` with `New(h *http.Client, opts ...Option)`: default nil to `http.DefaultClient` and apply options; verify a test constructs a client with options and reads back the expected defaults.

## 2. Verb methods and request building

- [x] 2.1 Implement `newRequest(ctx, method, url string, body []byte, headers http.Header)` using a fresh `bytes.NewReader` and copied headers; verify the POST method/body round-trip test passes.
- [x] 2.2 Implement `Get`, `Post`, `Put`, `Patch`, `Delete` as thin wrappers over `Do(ctx, method, path, body, opts...)`; verify each verb reaches the test server with the expected HTTP method.
- [x] 2.3 Layer per-request options over the client's stored config (clone then apply, so per-request `WithHeader` wins); verify a per-request header overrides a global one and the global default is not mutated.
- [x] 2.4 Remove the `Policy` struct and its dead `BaseURL`/`APIKey` fields; verify `grep -rn "Policy" pkg/apiclient` finds no references and the package builds.

## 3. Retry, timeout, and URL behavior

- [x] 3.1 Port the retry loop into `Do` (`maxAttempts < 1` means one attempt; nil `retryIf` retries transport errors only and returns any HTTP response as-is; intermediate bodies drained and closed; final attempt returned); verify the retry-then-success, no-retry-on-4xx, exhausted-returns-last, and cancelled-context tests pass.
- [x] 3.2 Keep `backoff`, `sleep`, and `drainClose` unchanged and apply the per-attempt timeout around request build and send; verify a test that hits a slow handler is aborted by `WithTimeout` and that backoff is capped.
- [x] 3.3 Verify an absolute request path bypasses `baseURL` (test a client configured with a base URL that GETs a different absolute test-server URL).

## 4. Documentation and package verification

- [x] 4.1 Update the package doc comment to describe the configured-client/verb model and remove the builder-closure description; verify `go vet ./pkg/apiclient` is clean.
- [x] 4.2 Run `gofmt -l pkg/apiclient` (expect no output) and `go test ./pkg/apiclient` (expect pass).

## Workflow follow-up

- Migrate `internal/webhooks/consumer.go`, `internal/payment/repositories/mastercard.go`, and `internal/app/container.go` to the new API in a follow-up change; `go build ./...` fails until that lands.
- Archive this change after review.
