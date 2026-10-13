# Tasks

## 1. Retry client

- [x] 1.1 Add `pkg/apiclient` (`Do` with `Policy{MaxAttempts, BaseBackoff, MaxBackoff, Timeout, RetryIf}`, exponential backoff with jitter, ctx-aware sleeps, stdlib only) and verify `go build ./...` passes
- [x] 1.2 Cover the client with httptest tests (attempt counts on 5xx-then-200, no retry on 4xx, backoff budget bounds, ctx cancel aborts, header passthrough) and verify `go test ./pkg/apiclient/ -v` passes

## 2. Mastercard repository

- [x] 2.1 Add `internal/payment/repository_card_network_master_card.go` implementing `Processor` (`Call`=PUT, `Status`=GET mapping, `Create`=POST, `Retryable` classification, Bearer auth, JSON bodies per design assumptions) and verify `go build ./...` passes
- [x] 2.2 Cover the repo with httptest tests (PUT path/body/key header, GET accepted/rejected mapping, POST creation, Retryable true/false matrix) and verify `go test ./internal/payment/ -v` passes

## 3. Service slim and wiring

- [x] 3.1 Slim `service.callProcessor` to single `Call` + `Status` reconcile (retire loop consts), re-target loop-count stub cases to equivalent outcomes, and verify `go test ./internal/payment/ -v` passes
- [x] 3.2 Add `MC_*` config fields with validation tests and wire client + repo in `container.go` (nil when base URL unset), and verify `go test ./...` and `go vet ./...` pass with handler status codes unchanged
