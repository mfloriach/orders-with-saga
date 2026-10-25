# Tasks

## 1. Broker infrastructure and configuration

- [x] 1.1 Add `RABBITMQ_URL` (required), `RABBITMQ_MAX_ATTEMPTS`, and retry configuration to `internal/config` with fail-fast validation, plus `.env.example` entries — verify: boot without URL exits non-zero; `go test ./internal/config/ -v` passes
- [x] 1.2 Wire broker into the app container: open broker, declare `payments.events` exchange, add readiness check and shutdown handling — verify: `go build ./...` passes; `/ready` reports the broker
- [x] 1.3 Add the relay worker binary (`cmd/worker`) with broker and app-DB wiring — verify: `go build ./...` produces both binaries

## 2. Outbox relay implementation

- [x] 2.1 Add outbox repository methods: mark processed, mark failed, increment attempts — verify: unit tests cover the state transitions
- [x] 2.2 Wire the outbox repository into the saga capture step — verify: capture writes a pending outbox row on successful capture
- [x] 2.3 Implement the relay worker: poll pending rows with `FOR UPDATE SKIP LOCKED`, publish generic captured-payment event, mark processed / retry / fail — verify: unit tests with fake outbox + fake broker

## 3. Integration verification

- [x] 3.1 Run `go test ./...` and `go vet ./...` — verify: green
- [x] 3.2 Live check against compose: captured payment → outbox pending → relay publishes → row marked processed — verify: observable via broker and DB state

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.
- Verify the archived result.
