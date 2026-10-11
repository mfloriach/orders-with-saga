# Design

## Context

`POST /payments` takes `idempotency_key` in the body (required, max 200); `service.Create(merchant_id, amount, idemKey)` enforces uniqueness via Redis fast-path + `UNIQUE(merchant_id, idempotency_key)` with a 24h Redis TTL. See `proposal.md` for why. Constraint: hard cut, handler-side resolution.

## Goals / Non-Goals

**Goals:**
- Header-sourced keys with derivation fallback; effective key echoed in responses.
- Dual windows: explicit 24h, derived ~10 min.

**Non-Goals:**
- Body-key backward compatibility, middleware extraction, service-layer spans, second-endpoint reuse.

## Decisions

- **Service-side `ResolveKey(header, merchantID, amount)`:** the service takes the raw header and resolves key+TTL itself, keeping all idempotency validation in one layer; the handler passes the header through untouched and echoes the effective key from the returned payment object. Empty/overlong headers (>200 chars) fall back to derivation rather than 400. Alternative (handler-side resolution) rejected: it split key logic across two layers for no benefit.
- **Derivation `drv:` + hex `SHA256(merchant_id:amount)`:** struct fields, not raw bytes — immune to JSON formatting differences between retries. `drv:` prefix distinguishes provenance in storage/logs.
- **Per-key TTL in service:** `Create` takes a TTL alongside the key (24h explicit, 10m derived). Redis `SET` uses it; the DB unique constraint stays the source of truth regardless of window. Alternative (single TTL) rejected: accepts the retry-vs-coincidence collision documented in explore.
- **Echo via `Idempotency-Key` response header:** set in handler from the resolved key on 201 and 409. Reuses the existing `ConflictError{Existing}` path unchanged.
- **No service signature churn beyond TTL:** merchant-exists check, ledger compensate, and conflict mapping untouched. Body struct drops the field; validator tags for it go with it.

## Risks / Trade-offs

- [Derived-key coincidence (same merchant+amount within 10 min)] → bounded by the short window; documented, accepted.
- [Hard cut breaks any existing client] → accepted: not in production.
- [Oversized header silently derived instead of 400] → deliberate: derivation is always a safe fallback.

## Migration Plan

- Land atomically (single change); update any client to send the header. No data migration (`idempotency_key` column stores the effective key either way).
- Rollback: revert binary; body-key contract restored.
