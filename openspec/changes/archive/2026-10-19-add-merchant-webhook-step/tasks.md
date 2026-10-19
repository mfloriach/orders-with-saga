# Tasks

## 1. Webhook step implementation

- [x] 1.1 Create `internal/payment/workflow/webhook.go` with the webhook step: `Name()`, `Do` (POSTs `webhooks.Event` to the merchant's `webhook_url` via `pkg/apiclient.Client` with `WithTimeout`/`WithRetry`), `Compensate` (re-runs `Do`), using an injected `MerchantLookup` for the URL — verify `go build ./internal/payment/workflow/`
- [x] 1.2 Add unit tests for the webhook step (successful POST, failed POST path, merchant with no webhook URL skips) — verify `go test ./internal/payment/workflow/`

## 2. Saga wiring

- [x] 2.1 Add `merchantStore` dependency to `workflow.NewWorkflow` and append the webhook step after `capture` in `registry.go` — verify `go build ./internal/payment/workflow/`
- [x] 2.2 Wire the merchant repository and an `apiclient.Client` into `NewWorkflow` in `internal/app/container.go` — verify `go build ./internal/app/`

## 3. Integration verification

- [x] 3.1 Run `go vet ./...` — verify no errors
- [x] 3.2 Run `go build ./...` — verify the whole module compiles
