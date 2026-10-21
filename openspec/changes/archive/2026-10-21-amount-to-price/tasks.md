# Tasks

## 1. Domain model

- [x] 1.1 Add `domain.Price{Amount int64; Coin string}` to `internal/domain/payment.go` and replace `Amount int64` with `Price Price` on `domain.Payment` and `domain.PaymentPending` — verify `go build ./internal/domain/...` succeeds
- [x] 1.2 Change `CardNetwork.Authorize(ctx, paymentID, amount int64, idemKey)` to keep its amount-only signature in `internal/domain/interfaces.go` — verify interface compiles with callers
- [x] 1.3 Change `MerchantPaymentSusccessfull.Amount int64` to `Price Price` in `internal/domain/merchant.go` — verify `go build ./internal/domain/...`

## 2. HTTP layer

- [x] 2.1 Replace `createBody.Amount int64` with `Price Price` and update `creator.Create(..., amount int64, ...)` to `Create(..., price domain.Price, ...)` in `internal/payment/handler.go`; add explicit `Price.Amount > 0` and `Price.Coin != ""` validation replacing `validate:"gt=0"` — verify `go build ./internal/payment/...`

## 3. Service

- [x] 3.1 Change `Service.Create(ctx, merchantID, amount int64, idemKey)` to take `price domain.Price` and build `PaymentPending{Price: price}` in `internal/payment/service.go` — verify `go build ./internal/payment/...`

## 4. Repository + card network

- [x] 4.1 Update `Scan(&p.Amount)` and the `INSERT ... p.Amount` to use `p.Price.Amount`/`p.Price` in `internal/payment/repositories/payments.go` — verify `go build ./internal/payment/repositories/...`
- [x] 4.2 Keep `MasterCardRepository.Authorize(..., amount int64, ...)` passing `price.Amount` to the simulator body in `internal/payment/repositories/mastercard.go` — verify `go build ./internal/payment/repositories/...`

## 5. Workflow

- [x] 5.1 Pass `p.Price.Amount` to `CardNetwork.Authorize` in `internal/payment/workflow/authorize.go` — verify `go build ./internal/payment/workflow/...`
- [x] 5.2 Set webhook `MerchantPaymentSusccessfull{Amount: wf.Pending.Price}` in `internal/payment/workflow/webhook.go` — verify `go build ./internal/payment/workflow/...`

## 6. Email

- [x] 6.1 Change `RefundInProgress.Amount int64` to `Price Price` and update both `"Amount": ...` template payloads to `price` objects in `internal/email/email.go` — verify `go build ./internal/email/...`
- [x] 6.2 Update four templates (`payment_success.html/.txt`, `refund_in_progress.html/.txt`) to render `{{.Price.Amount}}`/`{{.Price.Coin}}` — verify `go build ./internal/email/...` and templates still parse

## 7. Database + API contract

- [x] 7.1 Add migration adding `coin TEXT NOT NULL DEFAULT 'USD'` to `payments` in `migrations/app/` — verify the SQL is syntactically valid against the existing schema
- [x] 7.2 Change `amount` integer to `price {amount, coin}` object in `api/openapi.yaml` (PaymentCreate and Payment schemas) — verify the YAML parses

## 8. Verify whole change

- [x] 8.1 Run `go build ./...` and `go vet ./...` — both exit 0
- [x] 8.2 Run `openspec validate` for the change — validation passes
