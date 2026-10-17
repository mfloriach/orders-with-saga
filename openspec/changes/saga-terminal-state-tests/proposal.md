# Proposal

## Why

The saga's terminal payment state after each failure position is currently pinned only indirectly (service-level tests with fakes, worker settle tests). A steps-level table test drives the real `Registry` + `saga.New` through every failure position and asserts the terminal row state, so the state machine in `internal/payment/steps` is covered directly.

## What Changes

- One new test file in `internal/payment/steps` (test-only change; no production code modified):
  - Table rows fail the saga at each step (`insert-payment`, `authorize`, `capture` network error, `capture` `MarkCaptured` error with remote rejected/accepted, `webhook`) and assert the terminal row state after reverse unwind.
  - `insert-payment` failure asserts the explored decision: `completed` is empty, nothing compensates, the row is untouched.
  - Capture covers both sub-cases (network error → in-doubt/pending; `MarkCaptured` error → unwind to failed).
- Test-local fakes for `domain.PaymentRepository`, `domain.CardNetwork`, and `webhooks.Publisher`; no new seams, no fixture files.

## Capabilities

### New Capabilities

(none — test-only change)

### Modified Capabilities

(none — behavior unchanged; `skip_specs: true`)

## Impact

- New file: `internal/payment/steps/terminal_test.go` (name TBD at implementation).
- No API, migration, topology, or spec changes.
