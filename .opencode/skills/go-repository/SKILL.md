---
name: go-repository
description: Writes and reviews Go database repositories that map rows to domain objects and enforce state transitions. Use whenever adding or changing a Go repository or adapter (payment, order, user, etc.), writing database/sql SQL, mapping persistence models to domain types, handling unique-constraint/not-found/wrong-state errors, or editing files under internal/*/repositories. Use it even when the request is only "add a query" or "store this", because repository shape and error behavior are easy to get subtly wrong.
---

# Go Repository

Reference implementation: `internal/payment/repositories/payments.go`. Read it before writing or reviewing a repository; copy its shape and error style.

## Shape

- Keep the concrete type unexported: `type paymentDatabaseRepository struct{ db *sql.DB }`.
- Export the constructor and every method: `NewPaymentDatabaseRepository`, `New`, `Authorized`, `Captured`, `Failed`, `IsIdempotent`.
- Keep the repository stateless. It holds its connection (or a transaction-bound handle) and nothing that belongs to one request.
- Take `ctx context.Context` first and pass it to every `ExecContext`, `QueryContext`, and `QueryRowContext`. Never drop it, replace it, or use `context.Background()`.
- Keep persistence structs (`type payment struct`) private and local to the repository file.
- If per-call configuration is needed (transactions, timeouts), use variadic options such as `opts ...Option` and resolve them once per call. Do not add a setter or mutable repository state.

```go
type paymentDatabaseRepository struct{ db *sql.DB }

func NewPaymentDatabaseRepository(db *sql.DB) *paymentDatabaseRepository {
	return &paymentDatabaseRepository{db: db}
}

func (r *paymentDatabaseRepository) New(ctx context.Context, p *domain.PaymentPending) error {
	// ctx on every query; return domain errors, not driver errors.
}
```

## Domain objects, not database objects

- Return domain types (`domain.PaymentCaptured`, `domain.PaymentFailed`, ...), never `sql.Row`, `pgx.Rows`, or the private persistence struct.
- Scan into the private model when the row layout differs from the domain type, then translate explicitly before returning.
- Let callers depend on domain types only, so the persistence choice stays replaceable.

## State transitions

When methods represent lifecycle states (`Authorized`, `Captured`, `Failed`, refunds), the method name is the transition:

- Return the domain object for the state the transition produces:
  `PaymentPending` → `PaymentAuthenticated` → `PaymentCaptured`, or `PaymentFailed`.
- Bound every UPDATE by the allowed source state so concurrent or repeated calls cannot skip a state:

  ```sql
  UPDATE payments SET status = 'captured'
  WHERE id = $1 AND status = 'authorized'
  ```

- Treat `rows == 0` as "not found or wrong current state" and return an explicit, contextual error. Do not report success.
- If the domain uses a state machine and the legal source/target state or returned object is not clear, stop and ask the user which transition is intended. Do not invent statuses or transitions.

## Errors for every path

Handle each outcome explicitly and return an error for each failure path:

- Insert/update unique violation → domain sentinel (`domain.ErrConflict`, `ErrEmailTaken`) when one exists.
- `sql.ErrNoRows` on a lookup → the domain's not-found error, or a documented empty result when the contract says so (as `IsIdempotent` does).
- `rows == 0` on a transition → not found or invalid current state.
- `RowsAffected`, `Scan`, `Rows.Err`, and `Exec` failures → wrapped error.
- Never swallow an error, and never return success with a zero-value domain object.

## Errors with context

Wrap infrastructure and unexpected errors with the operation and the identifier, using `%w`:

```go
return domain.PaymentCaptured{}, fmt.Errorf("mark payment %q as captured: %w", p.ID, err)
```

Use `%w` (not `%v`) so callers can inspect the cause with `errors.Is` / `errors.As`. Keep domain sentinel errors unwrapped when they are the intended public contract.

## Checklist

Before finishing, verify:

- [ ] Concrete type unexported; constructor and methods exported.
- [ ] `ctx` first and propagated to every database call.
- [ ] Returns domain types only; persistence structs private.
- [ ] Every UPDATE constrained by the expected source state.
- [ ] Every outcome has an explicit, contextual error path.
- [ ] Errors wrapped with `%w` and the operation/ID.
- [ ] Unknown state-machine transitions were clarified with the user, not guessed.
