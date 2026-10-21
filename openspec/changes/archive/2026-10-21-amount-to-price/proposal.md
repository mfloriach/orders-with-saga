# Proposal

## Why

The `domain.Payment` and `domain.PaymentPending` models express money as a bare
`int64 Amount` with no currency. Every consumer silently assumes a single
implicit currency, and the API contract cannot say which money a payment is
denominated in. As the gateway supports more than one currency, amount must
become an explicit price object that pairs an amount with its coin.

## What Changes

- Introduce `domain.Price{amount int64, coin string}` in
  `internal/domain/payment.go`.
- Replace `Amount int64` with `Price Price` on `domain.Payment` and
  `domain.PaymentPending` (inherited by `PaymentAuthenticated`,
  `PaymentCaptured`, `PaymentFailed`).
- Update every consumer of `Amount`: the HTTP `createBody` + `creator`
  interface (`internal/payment/handler.go`), `Service.Create`
  (`internal/payment/service.go`), the repository scans/inserts
  (`internal/payment/repositories/payments.go`), the `CardNetwork.Authorize`
  seam (`internal/domain/interfaces.go`,
  `internal/payment/repositories/mastercard.go`,
  `internal/payment/workflow/authorize.go`), the merchant webhook payload
  (`internal/payment/workflow/webhook.go`, `internal/domain/merchant.go`),
  the email `PaymentSuccessfully` and `RefundInProgress` payloads and four
  templates (`internal/email/email.go`, `internal/email/templates/`), the
  `payments` migration (add `coin` column), and `api/openapi.yaml`.
- **BREAKING**: the JSON `amount` integer becomes a `price` object
  (`{amount, coin}`) in create requests and payment responses; clients must
  adapt.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `payment-processing`: the "Create payment" requirement now accepts a
  `price` object (amount + coin) instead of a currency-less integer amount,
  so the "no currency" clause is removed and the idempotency-key derivation
  input changes to include coin.

## Impact

- Code: `internal/domain/payment.go`, `internal/domain/merchant.go`,
  `internal/domain/interfaces.go`, `internal/payment/handler.go`,
  `internal/payment/service.go`, `internal/payment/repositories/payments.go`,
  `internal/payment/repositories/mastercard.go`,
  `internal/payment/workflow/authorize.go`,
  `internal/payment/workflow/webhook.go`, `internal/email/email.go`,
  `internal/email/templates/*.html`, `internal/email/templates/*.txt`,
  `migrations/app/002_payments.sql`, `api/openapi.yaml`.
- Out of scope: the ledger DB `amount` column (`migrations/ledger/`) has no Go
  writer in this service and is a separate concern; tests are untouched per
  the change request (no new tests, existing tests not fixed).
