# Design

## Context

Money is currently modeled as `Amount int64` on `domain.Payment` and
`domain.PaymentPending` (see proposal.md - What Changes for the full list of
consumers). The database column is `amount BIGINT NOT NULL CHECK (amount > 0)`
in `migrations/app/002_payments.sql`; the ledger DB has a separate `amount`
column with no Go writer in this service. The `CardNetwork.Authorize` seam
sends `{"amount": ...}` to an amount-only simulator, and email templates
render `{{.Amount}}`. Validation today relies on `validate:"gt=0"` on the
int64 body field, which will not compile once `Amount` becomes a struct.

See proposal.md - Why and proposal.md - Impact; see specs/payment-processing
for the requirement delta (amount → price object with coin).

## Goals / Non-Goals

**Goals:**
- Replace `Amount int64` with an explicit `Price {amount, coin}` object across
  domain, HTTP, repository, card-network seam, webhook, email, migrations, and
  OpenAPI.
- Keep the change local to the payments domain; no new services, stores, or
  dependencies.
- Keep existing tests untouched and unmodified per the change request.

**Non-Goals:**
- Currency conversion or FX rates.
- Changing the ledger DB `amount` column (no Go writer; separate concern).
- Wiring the not-yet-existent refund flow; `RefundInProgress` is updated for
  consistency only.
- New tests or fixes to existing tests.

## Decisions

### 1. JSON shape: nested `price` object (BREAKING)
`Payment`/`PaymentPending` get `Price Price` serialized as `{"amount": 2500,
"coin": "USD"}` under the `price` key, replacing the top-level `amount`
integer. Alternative considered: flat `amount` + `coin` siblings — rejected
because it does not satisfy "price as object". This is a breaking API change
(clients must adapt), documented in proposal.md.

### 2. `domain.Price{Amount int64; Coin string}`
Exported fields in `internal/domain/payment.go`; `Coin` is a free-form string
(expected ISO 4217, e.g. `"USD"`), not an enum. Alternative considered: a
`Coin` type or enum — rejected to avoid coupling this change to a currency
vocabulary and to keep DB CHECK constraints simple. Application validation
requires `Coin != ""` and `Amount > 0`.

### 3. CardNetwork seam stays amount-only
`Authorize(ctx, paymentID, amount int64, idemKey)` keeps its signature; the
workflow passes `p.Price.Amount` and the `coin` travels only inside the
domain/DB/email boundary. The simulator accepts `amount` only, so extending
the seam would be speculative. Revisit if the simulator ever accepts coin.

### 4. Validation moves from struct tag to explicit check
`createBody.Amount int64 validate:"gt=0"` cannot compile on a struct field.
The handler validates `Price.Amount > 0` and `Price.Coin != ""` explicitly
(before calling the service); `validate.Struct` no longer covers amount.

### 5. Migration: add `coin TEXT NOT NULL DEFAULT 'USD'` to `payments`
New migration adds the column with a default so existing rows backfill to `"USD"`.
No rollback column drop is planned (data-preserving default).

### 6. Ledger DB and tests untouched
`migrations/ledger/001_entries.sql` `amount` column has no Go writer in this
service and is out of scope. No new tests; existing tests are not fixed. Note
`sender_test.go` passes string `"$10.00"` for `Amount` into template data maps
whose keys stay valid, so those tests continue to pass (non-empty output
assertion only).

## Risks / Trade-offs

- [Existing rows get `coin='USD'` by default] → if any pre-existing payment was
  denominated in another currency, its coin is mislabeled; no backfill source
  exists, so `'USD'` is the best available default. Mitigation: document the
  assumption; revisit when multi-currency backfill is needed.
- [Templates render `{{.Price.Amount}}` / `{{.Price.Coin}}`] → test data maps
  lack a `Price` key, so rendered output shows `<no value>` for those fields;
  tests still pass since they only assert non-empty output. Visual regression
  possible in real emails until templates/data are aligned.
- [Breaking API] → clients sending `amount` as an integer receive 400s.
  Mitigation: communicate the contract change; versioning is out of scope.

## Migration Plan

1. Apply the new `payments` migration adding `coin TEXT NOT NULL DEFAULT 'USD'`.
2. Deploy code with `Price` object; clients send `price {amount, coin}`.
3. Rollback: revert code (drops `coin` usage); the `coin` column remains
   harmlessly unused until a follow-up removes it.

## Open Questions

None. The coin default ('USD'), amount-only seam, and validation approach are
decisions recorded above; none of them change the specs or task breakdown.
