---
name: api-and-interface-design
description: Guides stable API and interface design in Go. Use when designing Fiber HTTP endpoints, module boundaries, service interfaces, or any public Go contract. Use when defining request/response types, error mapping, validation at boundaries, or interface seams between packages.
---

# API and Interface Design (Go)

## Overview

Design stable, well-documented interfaces that are hard to misuse. Good interfaces make the right thing easy and the wrong thing hard. In Go this means HTTP endpoints built on Fiber, narrow interfaces at package seams, typed domain errors, and JSON contracts backed by struct tags.

This skill covers the design rules. For the mechanical endpoint recipe — handler skeleton, `apperr` mapping, table tests — see the `go-endpoints` skill; the two are meant to be applied together.

## When to Use

- Designing new HTTP endpoints or changing existing ones
- Defining a service interface or a seam between packages
- Adding fields to a request/response contract or a domain type
- Mapping service errors to HTTP status codes
- Establishing database constraints that back an API guarantee (uniqueness, idempotency)
- Changing any exported Go type or JSON tag another package or client depends on

## Core Principles

### Hyrum's Law

> With a sufficient number of users of an API, all observable behaviors of your system will be depended on by somebody, regardless of what you promise in the contract.

Every observable behavior becomes a de facto contract: JSON field names and whether a field is omitted, error `code` strings, status codes, response headers, ordering, and timing. Design implications:

- **Be intentional about what you expose.** A JSON tag is a promise; an echoed header is a promise; an error code is a promise.
- **Don't leak implementation details.** Never expose `err.Error()`, SQL, or internal type names to clients.
- **Plan for deprecation at design time.** See `deprecation-and-migration`.
- **Tests are not enough.** Contract tests won't save you from clients depending on undocumented behavior.

### The One-Version Rule

Avoid forcing consumers to choose between multiple versions of the same package or API. In Go, a breaking change to an exported type or a `/v2` route duplicates maintenance and splits consumers. Extend instead of fork: add optional fields, new endpoints, new methods on new interfaces. Reserve a major-version bump (`/v2` import path or route prefix) for changes you cannot make additively.

### 1. Contract First

Define the interface and the wire shape before implementing. In Go the contract is the exported interface plus the JSON tags — both are compiled and reviewed before the body exists.

```go
// domain/payment.go — vocabulary shared by every layer.
type PaymentStatus string

// PaymentService is the seam the transport layer depends on. The concrete
// *payment.Service satisfies it structurally; neither package imports the other.
type PaymentService interface {
	Create(ctx context.Context, merchantID string, price Price, idemHeader string) (*PaymentCaptured, error)
	Get(ctx context.Context, id string) (*PaymentCaptured, error)
	List(ctx context.Context, f ListFilter) ([]Payment, int, error)
}
```

The handler still declares the **subset it actually calls** (see [Consumer-Side Narrow Interfaces](#consumer-side-narrow-interfaces)). The domain contract states the full capability; the handler depends only on the slice it uses.

### 2. Consistent Error Semantics

Pick one error strategy and use it everywhere. In this codebase that means: services return Go errors (sentinels and typed errors); the handler maps them to `apperr` constructors; `apperr.Handler` renders the single JSON envelope.

```go
// Sentinel: callers branch on identity with errors.Is.
var ErrMerchantNotFound = errors.New("merchant not found")

// Typed error: carries the data the handler needs to build the response.
type ConflictError struct {
	PaymentId     string
	IdempotentKey string
}

func (e *ConflictError) Error() string { return "idempotency_conflict" }
```

Every error response has the same shape:

```json
{
  "code": "merchant_not_found",
  "title": "Merchant not found",
  "details": "No merchant exists with the given ID.",
  "request_id": "f3a1..."
}
```

`code` is stable and machine-readable, `title` is a fixed human string, `details` explains without leaking internals, and `fields` carries validation or conflict data. Headers that clients need (for example `Idempotency-Key`, `Retry-After`, `Conflict-Payment-ID`) are set by the `apperr` constructor, not ad hoc in the handler.

| Service outcome | Constructor | Status |
|---|---|---|
| Resource missing | `apperr.NotFound()` | 404 |
| Merchant missing | `apperr.MerchantNotFound()` | 404 |
| Duplicate / idempotency hit | `apperr.IdempotencyConflict()` / `apperr.EmailTaken()` | 409 |
| Bad input | `apperr.ValidationFailed(errs)` | 400 |
| Unparseable JSON | `apperr.InvalidJSON()` | 400 |
| Rate limited | `apperr.RateLimitedError()` | 429 |
| Upstream unavailable | `apperr.BadGateway()` | 502 |
| Anything else | `apperr.InternalError()` | 500 |

**Don't mix patterns.** If some endpoints panic, others return `nil, nil`, and others write the response directly, the consumer cannot predict behavior. One mapping switch per handler, most specific error first, catch-all last:

```go
p, err := h.svc.Create(c.Context(), b.MerchantID, b.Price, c.Get("Idempotency-Key"))
var conflict *domain.ConflictError
switch {
case errors.Is(err, merchant.ErrNotFound):
	return apperr.MerchantNotFound()
case errors.As(err, &conflict):
	c.Set("Idempotency-Key", conflict.IdempotentKey)
	return apperr.IdempotencyConflict(conflict.PaymentId)
case err != nil:
	return apperr.InternalError()
}
return c.Status(http.StatusCreated).JSON(p)
```

Match identity with `errors.Is` and type with `errors.As`. **Never** compare `err.Error()` strings, and never pass `err` through to the client.

### 3. Validate at Boundaries

Trust internal code. Validate only where untrusted input enters: HTTP handlers, message consumers, config loading, and third-party responses.

```go
func (h *Handler) Create(c *fiber.Ctx) error {
	var b createBody
	if err := c.BodyParser(&b); err != nil {
		return apperr.InvalidJSON()
	}
	if errs := validate.Struct(b); errs != nil {
		return apperr.ValidationFailed(errs)
	}

	// Past this line the body is valid; internal code trusts the type.
	p, err := h.svc.Create(c.Context(), b.MerchantID, b.Price, c.Get("Idempotency-Key"))
	// ... error mapping ...
}
```

**Struct tags are parsed literally.** `validate:"gte=0, lte=100"` (space after the comma) is not two rules: the validator reads a function named ` lte` and **panics** at request time. Write `validate:"gte=0,lte=100"` — no spaces after commas. Keep the tag set, the code's defaults, the OpenAPI bounds, and the tests saying the same numbers; four places that disagree are four different contracts.

**Every numeric input needs both bounds.** Every request field of type `int`, `int64`, `float64`, or another numeric type must carry a lower and an upper validator: `validate:"gte=1,lte=100"`. A lone `gte` or `lte` is an incomplete contract. Optional numeric fields keep both bounds behind `omitempty` (`validate:"omitempty,gte=1,lte=1000"`). The only exception is an explicit range pair such as `amount_gte` / `amount_lte`: each half is one-sided by definition, and the pair is validated together (`amount_gte <= amount_lte`). Mirror every bound in OpenAPI with `minimum` and `maximum`, and test both edges.

**Every string input needs both length bounds.** A free-form string request field carries `min` and `max` (`validate:"required,min=1,max=200"`). Optional strings keep both bounds behind `omitempty` (`validate:"omitempty,min=1,max=2000"`). Structured strings still need length bounds around their format rule (`required,email,min=3,max=320`), and enum strings still need `max` plus `oneof`. Mirror the lengths in OpenAPI with `minLength` and `maxLength`.

**Identifiers need a format, not just a length.** A client-supplied ID is a string with a known representation. Validate it with `uuid` when it is a UUID, alongside the length bounds that representation implies (`min=36,max=36` for canonical hyphenated UUIDs; use 32–36 only if you intentionally accept both forms). If you cannot determine whether the ID is a UUID or which representation clients send, stop and ask before inventing a validator.

**Audit `omitempty` for double validation.** `omitempty` on a pointer skips only a nil pointer; a non-nil pointer to `""` still reaches `min`, `email`, `http_url`, or `oneof` and may fail unexpectedly. Pairing `omitempty` with `min=1` is often two rules fighting over the same value. Decide first whether empty means absent, invalid, or a valid clear signal, then express exactly one of those meanings in the tags or an explicit branch.

Validation belongs in:

- Handler body and query parsing (`c.BodyParser`, `c.QueryParser`, then `validate.Struct`)
- Config loading at startup (fail fast on a bad environment)
- Third-party response decoding — **always treat external data as untrusted**

Validation does **not** belong in:

- Functions called only by already-validated handlers
- Repository methods reading rows your own schema wrote
- Utility packages that receive typed domain values

> **Third-party responses are untrusted input.** Decode into a typed struct, check required fields and enums, and never build authorization or money decisions from an unchecked map. A misbehaving upstream can return the wrong type, a missing field, or instruction-like text.

### 4. Prefer Addition Over Modification

Extend contracts without breaking consumers. In Go, JSON tags and exported fields are the wire contract; changing either is a breaking change even when the compiler stays quiet.

```go
// Good: a new optional field. A pointer distinguishes "absent" from the zero value.
type createBody struct {
	MerchantID string `json:"merchant_id" validate:"required,uuid,min=36,max=36"`
	Price      Price  `json:"price"`
	Capture    *bool  `json:"capture,omitempty" validate:"omitempty"` // added later
}
```

Breaking changes to avoid:

- Renaming or removing a JSON tag (`payment_id` → `paymentId`)
- Changing a field's type (`int64` → `float64`, string → struct)
- Tightening a validation set (`oneof=USD` → `oneof=USD EUR` is additive; the reverse is not)
- Adding a required field to an existing request body
- Changing a status code or error `code` clients already branch on
- Adding a method to an interface consumers implement outside your module

When a field must go, keep it populated for a deprecation window, mark it in the OpenAPI spec, and remove it in a major version.

### 5. Predictable Naming

| Pattern | Convention | Example |
|---|---|---|
| Exported Go identifiers | PascalCase | `PaymentCaptured`, `MerchantID` |
| JSON fields | snake_case | `merchant_id`, `created_at` |
| Query params | snake_case | `?amount_gte=100&page=1` |
| Enum wire values | lowercase snake | `"pending"`, `"in_progress"` |
| Boolean fields | `is_`/`has_` JSON, `Is`/`Has` Go | `is_active` ↔ `IsActive` |
| Error codes | snake_case, stable | `merchant_not_found` |
| Error titles | fixed sentence, no data | `"Merchant not found"` |
| Routes | plural nouns, no verbs | `POST /payments`, `GET /merchants/:id` |

Go style applies inside the module: short receiver names, `ID` not `Id` in new Go identifiers, error variables prefixed `Err`, and one package-level vocabulary (do not mix `coin` and `currency` for the same concept). Existing exported names (for example `domain.ConflictError.PaymentId`) are part of the contract — rename them only through the breaking-change process, not as a drive-by cleanup.

### 6. Honour an Idempotency Key

Accepting `Idempotency-Key` is the contract. Honouring it is the implementation, and it is where money is lost. A key the server accepts but handles carelessly is worse than no key, because the client now believes retrying is safe.

**Derive the key from the intent, not the attempt.** The key must be stable across retries of one intent and different across distinct intents:

```go
uuid.NewString()                       // ✗ new key per attempt — every retry charges again
fmt.Sprintf("%s:%d", merchantID, amt)  // ✗ two legitimate $50 charges collapse into one
fmt.Sprintf("%s:%d", orderID, time.Now().UnixNano()) // ✗ a timestamp is a UUID in a hat

c.Get("Idempotency-Key")               // ✓ client generates once, reuses on retry
"charge:v1:" + orderID                 // ✓ derived from an immutable identifier
```

The key comes from the client or the initiating event — never from the layer doing the retrying. A request with no header is a **new intent** and must be documented as not retry-safe; do not silently dedupe keyless requests against each other.

**Claim atomically. A check followed by an act is a race:**

```go
// ✗ TOCTOU: two concurrent retries both read "not seen", both charge.
if id, _ := repo.IsIdempotent(ctx, merchantID, key); id != "" {
	return replay(id)
}
repo.New(ctx, pending)

// ✓ let a unique constraint pick the winner, in one statement.
_, err := db.Exec(ctx, `
	INSERT INTO payments (id, merchant_id, idempotency_key, request_hash, state)
	VALUES ($1, $2, $3, $4, 'in_progress')`,
	pending.ID, merchantID, key, requestHash)

var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
	return replayOrReject(ctx, merchantID, key, requestHash)
}
if err != nil {
	return fmt.Errorf("claim idempotency key: %w", err)
}
```

The unique index on `(merchant_id, idempotency_key)` **is** the mechanism. A store that cannot enforce uniqueness in one operation cannot back this guarantee. A `SELECT` followed by an `INSERT` is not a guard.

**Guard the payload.** The same key with a different body is a client bug and must fail loudly rather than replaying the first response:

```go
if existing.RequestHash != requestHash {
	return apperr.IdempotencyPayloadMismatch() // new constructor, stable code
}
```

`request_hash` is a hash of the canonical request body (or the fields that change the effect), stored in the same insert as the key.

**Decide what an in-flight duplicate gets.** The first request is still running when the second arrives — the common case under retry storms:

| Strategy | Response | Use when |
|---|---|---|
| Reject | `409 Conflict` | Client can retry later; simplest and safest |
| Wait | Block for the result, bounded by the request context | Caller needs it synchronously |
| Return pending | `202 Accepted` + status URL | Long-running effects |

Never let the second caller through because the first "seems stuck". A stalled attempt whose fate is unknown is exactly when duplicating costs most.

**Every outbound call has three outcomes: success, failure, and unknown.** A timeout tells you nothing about whether the effect applied. Record the intent before calling out — a `state = 'in_progress'` row and a remote idempotency key — so a crash between request and response leaves evidence to reconcile later, rather than a silently retried charge.

**Set retention from the longest retry chain**, not from disk cost. Keys must outlive every path that can re-deliver the same intent, including a dead-letter queue replayed a week later and any provider dispute window. A 24-hour key TTL behind a 7-day DLQ is a duplicate waiting to happen.

## Go Interface Patterns

### Consumer-Side Narrow Interfaces

Define the interface where it is consumed, and make it as small as the caller needs. The handler never imports repositories, SQL, or HTTP clients; it depends on a one-method seam that tests can stub.

```go
// creator is the subset of Service this handler calls (stubbed in tests).
type creator interface {
	Create(ctx context.Context, merchantID string, price domain.Price, idemHeader string) (*domain.PaymentCaptured, error)
}

type Handler struct{ svc creator }

func NewHandler(svc creator) *Handler { return &Handler{svc} }
```

- Accept interfaces, return concrete structs.
- Don't declare an interface until there is a seam: a second implementation, a test stub, or a package boundary.
- A 12-method interface where the caller uses one method is a coupling bug. Widen the seam when a new capability is genuinely needed — never reach around it.
- Keep parameters primitive or small structs; the service translates them into queries.
- No `any`/`interface{}` in a domain contract. Decode untyped external JSON at the edge, then convert.

### Typed Enums, Not Bare Strings

```go
type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusAuthorized PaymentStatus = "authorized"
	PaymentStatusCaptured   PaymentStatus = "captured"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusRefunded   PaymentStatus = "refunded"
)

func (s PaymentStatus) Valid() bool {
	switch s {
	case PaymentStatusPending, PaymentStatusAuthorized, PaymentStatusCaptured, PaymentStatusFailed, PaymentStatusRefunded:
		return true
	}
	return false
}
```

A distinct type stops undeclared values reaching a transition or a query. Mirror the vocabulary in the database `CHECK` constraint and in the OpenAPI enum, and extend all three together.

### Sealed Variants When Payloads Differ

Use a typed string plus a `switch` when every state carries the same data. Use a sealed interface only when variants carry different payloads:

```go
type PaymentEvent interface{ isPaymentEvent() }

type Authorized struct {
	AuthorizationKey string
	AuthorizedAt     time.Time
}
func (Authorized) isPaymentEvent() {}

type Captured struct {
	PaidAt time.Time
}
func (Captured) isPaymentEvent() {}
```

The unexported method seals the set to this package; a new variant is a compile-time prompt to update every `switch`.

### Input/Output Separation

```go
// Input: exactly what the caller may send. Unexported and handler-local.
type createBody struct {
	MerchantID string       `json:"merchant_id" validate:"required,uuid,min=36,max=36"`
	Price      domain.Price `json:"price"`
}

// Output: the domain type, including server-generated fields.
type PaymentCaptured struct {
	ID             string    `json:"id"`
	MerchantID     string    `json:"merchant_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Status         string    `json:"status"`
	PaidAt         time.Time `json:"paid_at"`
}
```

Never bind a request directly into a domain type that also has server-owned fields — it lets a client set `id`, `status`, or timestamps.

### Named ID Types

```go
type MerchantID string
type PaymentID string
```

Named types prevent passing a `PaymentID` where a `MerchantID` is expected. Convert explicitly at the boundary (`domain.MerchantID(b.MerchantID)`) so the conversion is visible. Where a field stays `string` for compatibility, keep the name explicit (`paymentID`, not `id`) and document the format.

## HTTP Endpoint Patterns (Fiber)

### Resource Design

```
GET    /payments              → List payments (filters + pagination)
POST   /payments              → Create a payment
GET    /payments/:id          → Get a payment
PATCH  /payments/:id          → Partially update a payment
DELETE /payments/:id          → Delete (idempotent; succeeds if already gone)

GET    /merchants/:id/payments → Sub-resource list
```

Plural nouns, no verbs, no `/getUser`, no `/createPayment`. Register routes in `internal/app/server.go` behind the shared middleware, and keep public business routes under the rate limiter.

### Handler Shape

Every handler parses, validates, calls exactly one service method, maps errors, and responds. Nothing else lives there.

```go
func (h *Handler) Create(c *fiber.Ctx) error {
	var b createBody
	if err := c.BodyParser(&b); err != nil {
		return apperr.InvalidJSON()
	}
	if errs := validate.Struct(b); errs != nil {
		return apperr.ValidationFailed(errs)
	}

	p, err := h.svc.Create(c.Context(), b.MerchantID, b.Price, c.Get("Idempotency-Key"))
	var conflict *domain.ConflictError
	switch {
	case errors.Is(err, merchant.ErrNotFound):
		return apperr.MerchantNotFound()
	case errors.As(err, &conflict):
		c.Set("Idempotency-Key", conflict.IdempotencyKey)
		return apperr.IdempotencyConflict(conflict.PaymentID)
	case err != nil:
		return apperr.InternalError()
	}

	c.Set("Idempotency-Key", p.IdempotencyKey)
	return c.Status(http.StatusCreated).JSON(p)
}
```

Rules:

- Malformed JSON is always `apperr.InvalidJSON()`; validation failures are always `apperr.ValidationFailed(errs)`.
- One service call. No business branches, repository calls, or outbound HTTP in the handler.
- One error-mapping switch, most specific first, `err != nil` catch-all last.
- Never leak `err.Error()`; headers clients need are set by `apperr` constructors.

### File Layout and Declaration Order

A handler file is read top-to-bottom. Keep declarations in dependency order and
never interleave the `Handler` type with request types, helpers, or methods. The
canonical layout is:

1. package and imports
2. constants (unexported, holding the real default)
3. the consumer-side service interface
4. request bodies and param/query structs
5. response envelopes and shared DTOs
6. `Handler` and `NewHandler`
7. handler methods, in the same order the routes are registered
8. helper functions, at the very end of the file

```go
const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 100
)

// 1. Seam
type merchantService interface {
	Register(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error)
	Get(ctx context.Context, id string) (*domain.Merchant, error)
	List(ctx context.Context, limit, offset int, orderBy, order string) ([]domain.Merchant, int, error)
	Update(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error)
	Delete(ctx context.Context, id string) error
}

// 2. Request shapes
type createBody struct{ ... }
type updateBody struct{ ... }
type idParam struct{ ... }
type listQuery struct{ ... }

// 3. Response shapes
type Pagination struct{ ... }
type ListResponse[T any] struct{ ... }

// 4. Handler and constructor
type Handler struct{ svc merchantService }

func NewHandler(svc merchantService) *Handler { return &Handler{svc} }

// 5. Methods, matching the route table order
func (h *Handler) Create(c *fiber.Ctx) error { ... }
func (h *Handler) List(c *fiber.Ctx) error   { ... }
func (h *Handler) Get(c *fiber.Ctx) error    { ... }
func (h *Handler) Update(c *fiber.Ctx) error { ... }
func (h *Handler) Delete(c *fiber.Ctx) error { ... }

// 6. Helpers, at the end of the file
func merchantID(c *fiber.Ctx) (string, error) { ... }
```

Why this order:

- Types are defined before the code that references them, so a reviewer reads
  the contract first and the behaviour second.
- `Handler` and its constructor sit next to the methods they own, not above the
  request types. Helper functions come last, after the handler behaviour.
- One method order — mirrored in `server.go` — makes route/handler drift obvious
  in review. If `server.go` registers `List` before `Get`, the file declares
  `List` before `Get`.
- Constants are handler-local implementation details: `defaultPage`, not
  `DEFAULT_PAGE`. Exported SCREAMING_SNAKE constants leak into the package API
  and are not Go naming.

Renaming a type is a contract change. In the same edit, update its doc comment,
every swag annotation, the OpenAPI schema name, any aliases, and the tests. A
comment or annotation that still names the old type is a broken contract, not a
cosmetic bug.

### Error Envelope

`apperr.Handler` (wired as Fiber's `ErrorHandler`) is the only place errors become responses. It emits `code`, `title`, `details`, merges `fields`, injects `request_id`, and applies stored headers. Add a constructor in `pkg/apperr/errors.go` when a new meaning appears; give it a stable snake_case `code`, a fixed `title`, and a `details` string with no dynamic content.

### Pagination

Every list endpoint accepts `page`, `limit`, `order_by`, and `order`, and returns the same envelope. Parse, default, then validate in the handler:

```go
type listQuery struct {
	Page      int    `query:"page"     validate:"gte=1,lte=100"`
	Limit     int    `query:"limit"    validate:"gte=1,lte=50"`
	Status    string `query:"status"   validate:"omitempty,oneof=pending authorized captured failed refunded,min=1,max=20"`
	OrderBy   string `query:"order_by" validate:"omitempty,oneof=created_at name,min=1,max=32"`
	Order     string `query:"order"    validate:"omitempty,oneof=asc desc,min=3,max=4"`
	AmountGte *int64 `query:"amount_gte" validate:"omitempty,gte=0"`
	AmountLte *int64 `query:"amount_lte" validate:"omitempty,gte=0"`
}
```

```go
var q listQuery
if err := c.QueryParser(&q); err != nil {
	return apperr.ValidationFailed(map[string]string{"query": "malformed query string"})
}
// Defaults first: an absent page/limit/order parses as zero, so validate afterwards.
if q.Page == 0 {
	q.Page = 1
}
if q.Limit == 0 {
	q.Limit = 20
}
if q.OrderBy == "" {
	q.OrderBy = "created_at"
}
if q.Order == "" {
	q.Order = "desc"
}
if errs := validate.Struct(q); errs != nil {
	return apperr.ValidationFailed(errs)
}
if q.AmountGte != nil && q.AmountLte != nil && *q.AmountGte > *q.AmountLte {
	return apperr.ValidationFailed(map[string]string{"amount_gte": "must be <= amount_lte"})
}
```

- **`page` defaults to 1, never 0.** With `offset = (page-1)*limit`, a zero
  default becomes a negative `OFFSET`, which PostgreSQL rejects.
- **Bound, don't silently mutate.** Numeric request fields carry both `gte` and
  `lte`; values outside the range are rejected with 400. Do not also clamp: an
  `lte` bound and a clamp cannot both define the contract. If a product decision
  later changes to clamping, remove `lte` and update the OpenAPI description and
  tests in the same change.
- **Ordering is part of the contract.** Every list endpoint accepts `order_by`
  (an allowlisted column) and `order` (`asc`/`desc`), defaults both, and passes
  them to the service. Never interpolate client text into `ORDER BY`: whitelist
  values with `oneof` and map them to known columns.
- **Initialize an empty page to `[]T{}`** (`if items == nil { items = []T{} }`)
  so JSON is `[]`, not `null`.

```json
{
  "data": [],
  "pagination": { "page": 1, "limit": 20, "total_items": 142, "total_pages": 8 }
}
```

```go
type Page[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}
```

Defaults are input normalization (handler work); ranges are validation (also handler work). The service owns filtering semantics and total counts.

### Filtering

snake_case query params, explicit ranges for anything ordered, and explicit ordering:

```
GET /payments?status=captured&merchant_id=m-123&amount_gte=100&amount_lte=5000&order_by=created_at&order=desc
```

Use pointers for optional numeric ranges so "absent" is distinguishable from zero. Reject inverted ranges (`gte > lte`) as a validation error. `order_by` must be an allowlisted column and `order` must be `asc` or `desc`; map them to SQL identifiers in code rather than concatenating client input.

### Partial Updates (PATCH)

Accept a partial body; pointer fields distinguish "not provided" from "set to zero":

```go
type updateBody struct {
	Name *string `json:"name" validate:"omitempty,min=1,max=200"`
}

// Only fields whose pointer is non-nil are written; everything else is preserved.
// Audit omitempty before adding it: on a pointer it skips nil, not "".
```

An empty PATCH body changes nothing and returns the current resource. PUT (full replacement) is only appropriate when the client truly owns every field.

## OpenAPI with Scalar

Annotations live on the handler; the committed spec is `api/openapi.yaml` and is served as-is. The spec is reviewed with the code and never generated ad hoc into a second location.

```go
// CreatePayment godoc
// @Summary      Create a payment
// @Tags         payments
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key header string false "Idempotency key"
// @Param        body body createBody true "Payment"
// @Success      201  {object}  domain.PaymentCaptured
// @Failure      400  {object}  apperr.AppError
// @Failure      404  {object}  apperr.AppError
// @Failure      409  {object}  apperr.AppError
// @Failure      502  {object}  apperr.AppError
// @Router       /payments [post]
func (h *Handler) Create(c *fiber.Ctx) error {
```

- `internal/app/docs.go` serves `api/openapi.yaml` at `GET /openapi.yaml` and Scalar at `GET /docs`. Reuse `openAPIHandler` and `docsHandler`; do not add a second spec file or Swagger UI.
- Every new route and every changed request/response type updates `api/openapi.yaml` in the same commit: path, schema, required list, enum, status codes, and error responses.
- List query parameters document `page`, `limit`, `order_by`, and `order`; their `minimum`/`maximum`/`enum` values match the validator tags. String schemas carry `minLength`/`maxLength`, and ID fields carry `format: uuid` when applicable.
- CI must fail if a route registered in `internal/app/server.go` has no corresponding path entry, or if the YAML no longer parses. A small Go test that parses the spec and walks the route table is enough.
- Eyeball new endpoints in `/docs` before merge.

## Handler Tests

Test through `app.Test` with a stubbed service — no database, no network. Table tests cover validation, error mapping, and the happy path.

```go
h := NewHandler(stubService{create: tc.stub})
app := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})
app.Post("/payments", h.Create)

req := httptest.NewRequest("POST", "/payments", strings.NewReader(tc.body))
req.Header.Set("Content-Type", "application/json")
resp, err := app.Test(req)
// assert status, body["code"], and — for validation — the field keys under fields["errors"]
```

One row per case: malformed JSON, validation failure, service not-found, service conflict, upstream failure, success. Validation cases cover both edges of every numeric and string bound, malformed IDs, and `omitempty` behaviour. List endpoints additionally cover default `page`/`limit`, both lower and upper limits, `order_by`/`order`, inverted ranges, and the pagination envelope. Assert on the public contract (status, `code`, JSON field names), not internal error strings.

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "We'll document the API later" | The interface and JSON tags ARE the documentation. Define them first. |
| "We don't need pagination for now" | You will the moment someone has 100+ rows. Add it from the start. |
| "PATCH is complicated, let's just use PUT" | PUT requires the full object every time. PATCH is what clients actually want. |
| "We'll version the API when we need to" | Breaking changes without a version break consumers. Design for extension from the start. |
| "Nobody uses that undocumented behavior" | Hyrum's Law: if it's observable, somebody depends on it. Treat every public behavior as a commitment. |
| "We can just add a `/v2` package" | Multiple versions multiply maintenance and split consumers. Prefer the One-Version Rule. |
| "Internal APIs don't need contracts" | Internal consumers are still consumers. Narrow interfaces prevent coupling and enable parallel work. |
| "The handler can just query the DB, it's one line" | That line couples transport to storage and makes the handler untestable without a database. One service call. |
| "We compare the error string, it's quicker" | Error text is not a contract. Use `errors.Is`/`errors.As`; strings change silently. |
| "Wrapping with `%w` is enough" | The handler must map the error to an `apperr` constructor. An unmapped error becomes a 500 with leaked detail risk. |
| "Validation in the service is fine" | The service is called by trusted code too. Validate at the edge; keep internals trusting types. |
| "`gte` is enough for a count, limit, or amount" | A lone lower bound accepts absurd values and leaves clients guessing the ceiling. Add `lte`, mirror it in OpenAPI, and test both edges. |
| "A string only needs a max length" | A lone `max` accepts empty and one-character values where the domain requires substance. Add `min`, mirror it in OpenAPI, and test both edges. |
| "Any non-empty string is a valid ID" | An ID has a representation. Validate UUIDs with `uuid`; if the format is unknown, ask instead of guessing. |
| "`omitempty` makes a field optional" | On a pointer it skips only nil. A non-nil pointer to `""` still runs the other validators and can fail twice over. Decide what empty means first. |
| "Sorting is an implementation detail" | `order_by` and `order` are public contract. Allowlist columns with `oneof`, map them to SQL in code, and never concatenate client input. |
| "Accepting the Idempotency-Key header is enough" | The header is the contract; claiming the key atomically is the implementation. A key you accept but don't honour tells the client retrying is safe when it isn't. |
| "Our queue guarantees exactly-once delivery" | No queue does across a consumer crash — the ack and your side effect are not one transaction. Design for at-least-once with idempotent processing. |
| "Duplicate requests are rare" | They're *correlated*. Retries spike exactly when a dependency is degraded — the moment duplicates are most likely and most expensive. |
| "A `SELECT` then `INSERT` is basically atomic" | It's a TOCTOU race. Only a unique constraint claimed in one statement is a guard. |
| "Go compiles it, so declaration order doesn't matter" | Reviewers read top-down. Interleaving the Handler, request types, and helpers hides the contract and invites stale references. Follow the file layout. |

## Red Flags

**Contracts and handlers**

- Endpoints that return different shapes depending on conditions
- Inconsistent error envelopes, or handlers writing error JSON directly
- Handler imports `database/sql`, `pgx`, an HTTP client, or a repository package
- More than one service call, or business branching, inside a handler
- `err.Error()` or SQL text reaching the client
- Errors matched with `strings.Contains` or `==` on the message
- Missing `validate` tags on a body or query struct
- A string request field missing `min` or `max`, or an ID string missing its `uuid`/format validator
- A numeric request field (`int`, `int64`, `float64`, ...) with only one bound — `gte` without `lte`, or the reverse — outside an explicit paired range filter
- `omitempty` stacked with a validator that rejects the empty value, so the field is validated twice with contradictory meanings
- List endpoints without `page`/`limit`, or a `limit` without both lower and upper bounds
- List endpoints without `order_by` and `order`, or an unallowlisted `order_by` that reaches SQL
- Numeric or string OpenAPI fields missing `minimum`/`maximum` or `minLength`/`maxLength`, or bounds that disagree with the validator tags
- PATCH fields that aren't pointers, so "absent" overwrites with zero
- Request bodies bound directly into a domain type with server-owned fields
- JSON tags renamed/removed, or field types changed in place
- `any`/`interface{}` in a domain contract
- A wide interface declared next to its implementation instead of a narrow one at the consumer
- `Handler` declared above the request/response types, or helpers and methods interleaved with type declarations
- Helper functions declared before the `Handler` type or interleaved with handler methods instead of at the end of the file
- Handler methods in a different order than the routes in `server.go`
- Handler-local constants exported in SCREAMING_SNAKE (`DEFAULT_PAGE`) instead of `defaultPage`
- `defaultPage = 0` (or any default that makes `(page-1)*limit` negative)
- A space after the comma in a `validate` tag (`"gte=1, lte=100"`) — the validator panics on ` lte`
- A pagination bound in the tag that contradicts the OpenAPI maximum, or an `lte` bound combined with a clamp in the handler
- A renamed response type whose doc comment, swag annotation, or OpenAPI schema still uses the old name
- An empty list response serializing as `null` because the nil slice was not normalized to `[]T{}`

**Idempotency**

- A `SELECT` for an idempotency key followed by an `INSERT` — that's a race, not a guard
- An idempotency key derived from a UUID, timestamp, or anything regenerated per attempt
- The same key accepted with a different body, silently returning the first response
- An outbound call made before the intent is persisted, so a timeout has no record
- A key retention window shorter than the longest retry path, including dead-letter replay
- Keyless requests deduped against each other

**Docs**

- A route in `server.go` with no path in `api/openapi.yaml`
- A second spec file, or generated output committed next to the committed spec
- Swagger UI instead of Scalar, or docs served from a different path

## Verification

After designing or changing an API:

- [ ] Every endpoint has typed input and output structs with complete tags
- [ ] Every handler parses, validates, calls one service method, maps errors, responds
- [ ] Error responses follow the single `apperr` envelope with stable `code`/`title`
- [ ] Service errors are mapped with `errors.Is`/`errors.As`, catch-all last; nothing leaks
- [ ] Validation happens at boundaries only; internals trust typed values
- [ ] Every numeric request field has both lower and upper validator bounds; explicit range pairs validate `gte <= lte`
- [ ] Every string request field has both `min` and `max`; IDs carry `uuid` or another determined format validator
- [ ] `omitempty` usage has been audited: empty is deliberately absent, invalid, or a clear signal — not contradictory
- [ ] List endpoints support `page`/`limit` with both bounds and `order_by`/`order` with allowlisted values
- [ ] OpenAPI numeric and string bounds (`minimum`/`maximum`, `minLength`/`maxLength`) match the validator tags
- [ ] New fields are additive and optional (pointer or `omitempty`), JSON tags unchanged
- [ ] Naming follows Go/JSON/query conventions consistently across all endpoints
- [ ] Handler depends on a narrow consumer-side interface; tests stub it
- [ ] Table tests cover validation, error mapping, and success through `app.Test`
- [ ] Handler file follows the canonical declaration order (constants → interface → request/response types → `Handler` → methods → helpers at the end)
- [ ] Handler methods appear in the same order as the routes in `server.go`
- [ ] Handler-local constants are unexported and `defaultPage` is 1
- [ ] `validate` tags contain no spaces after commas; defaults, bounds, OpenAPI `minimum`/`maximum`/`minLength`/`maxLength`, and tests agree
- [ ] Renamed types have no stale comment, annotation, alias, or OpenAPI name
- [ ] Empty list responses serialize as `[]`, not `null`
- [ ] State-changing endpoints either honour an idempotency key or are documented as unsafe to retry
- [ ] The key is claimed in one atomic statement, guarded by a unique constraint
- [ ] A reused key with a different payload fails loudly rather than replaying the wrong response
- [ ] The in-flight-duplicate response is a deliberate choice (409, wait, or 202)
- [ ] Intent is persisted before every outbound call, so "unknown" outcomes are reconcilable
- [ ] Key retention outlives the longest retry path, including dead-letter replay
- [ ] `api/openapi.yaml` is updated in the same commit and served at `/openapi.yaml`; `/docs` checked
- [ ] CI fails when a registered route has no spec entry
