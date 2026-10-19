# Proposal

## Why

Merchants need to be notified when a payment completes so their systems can react (fulfill orders, update balances, reconcile). Today payment creation publishes an async webhook event to a broker, but there is no synchronous merchant notification as part of the creation flow. Adding a merchant webhook call as a saga step lets merchants receive completion notification inline with the payment response, and a failed notification unwinds the payment so no money is captured without the merchant being told.

## What Changes

- Add a `webhook` saga step after `capture` in `internal/payment/workflow/` that POSTs a `webhooks.Event` to the merchant's registered `webhook_url` using `pkg/apiclient` with retries and a per-attempt timeout.
- The webhook step's `Compensate` re-runs `Do` (re-POSTs the merchant notification), mirroring the existing `authorize` compensation pattern.
- Wire a merchant-store lookup (`FindByID`) and an `apiclient.Client` into `workflow.NewWorkflow` so the step can resolve the merchant's webhook URL.
- A failed webhook step marks the payment `failed`, returns an error to the client, and compensates `capture` then `authorize` in reverse order.
- No new config: reuse the existing `MC_*` timeout/retry settings for the webhook client.

## Capabilities

### New Capabilities

None. The behavior (merchant webhook notification on payment completion) is already owned by existing capabilities; no new durable capability boundary is introduced.

### Modified Capabilities

- `payment-processing`: the `Create payment` requirement now includes a synchronous merchant webhook call within the creation saga. Webhook failure marks the payment `failed`, returns `500` to the client, and compensates prior saga steps. The webhook delivery now affects the returned response, where it previously did not.
- `webhook-delivery`: in addition to async background delivery via the broker, payment creation now performs a synchronous merchant webhook call that can block the payment response. The guarantee "an unreliable merchant endpoint never blocks payment responses" no longer holds for the sync path.

## Impact

- `internal/payment/workflow/webhook.go` (new): the webhook step implementation.
- `internal/payment/workflow/registry.go`: new `webhook` step appended to the saga; `NewWorkflow` gains a merchant-store dependency.
- `internal/app/container.go`: wire the merchant repository and an `apiclient.Client` into `NewWorkflow`.
- `internal/payment/service.go`: unchanged behavior at the service layer (webhook failure surfaces as saga failure already handled).
- `pkg/apiclient`: reused as-is; no changes.
- Openspec deltas: `payment-processing` and `webhook-delivery` requirement changes.
