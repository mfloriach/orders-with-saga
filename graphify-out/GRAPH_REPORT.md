# Graph Report - paymentgateway  (2026-10-16)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 458 nodes · 1547 edges · 21 communities (16 shown, 5 thin omitted)
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 62 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `4429113a`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- worker/main.go
- Build
- apiclient_test.go
- testing.T
- AppError
- Broker
- Payment
- saga_test.go
- Merchant
- NewService
- Consumer
- log/slog.Logger
- context.Context
- Consumer
- Intent
- health_test.go
- InDoubtError
- RunError
- UnknownStepError
- Health
- paymentgateway

## God Nodes (most connected - your core abstractions)
1. `Payment` - 28 edges
2. `Build()` - 20 edges
3. `Container` - 15 edges
4. `Broker` - 15 edges
5. `PaymentRepository` - 15 edges
6. `Merchant` - 15 edges
7. `main()` - 14 edges
8. `Intent` - 13 edges
9. `AppError` - 13 edges
10. `Event` - 13 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewConsumer()`  [EXTRACTED]
  cmd/worker/main.go → internal/compensation/consumer.go
- `main()` --calls--> `OpenBroker()`  [EXTRACTED]
  cmd/worker/main.go → internal/config/rabbitmq.go
- `main()` --calls--> `Registry()`  [EXTRACTED]
  cmd/worker/main.go → internal/payment/steps/registry.go
- `main()` --calls--> `NewConsumer()`  [EXTRACTED]
  cmd/worker/main.go → internal/webhooks/consumer.go
- `main()` --calls--> `NewEmailConsumer()`  [EXTRACTED]
  cmd/worker/main.go → internal/webhooks/consumer.go

## Import Cycles
- None detected.

## Communities (21 total, 5 thin omitted)

### Community 0 - "worker/main.go"
Cohesion: 0.14
Nodes (3): User, createBody, createBody

### Community 1 - "Build"
Cohesion: 0.06
Nodes (31): Container, main(), main(), Build(), NewCardNetwork(), readyChecks(), configure(), NewServer() (+23 more)

### Community 2 - "apiclient_test.go"
Cohesion: 0.10
Nodes (24): script, decodePayment(), isTransient(), NewMasterCardRepository(), statusError(), backoff(), drainClose(), Client (+16 more)

### Community 3 - "testing.T"
Cohesion: 0.16
Nodes (35): Handler, NewHandler(), doPost(), okStub(), TestCreateEchoesStoredStatus(), TestCreateErrors(), TestCreateSuccess(), TestHeaderPassedToService() (+27 more)

### Community 4 - "AppError"
Cohesion: 0.11
Nodes (19): AppError, Handler, NewHandler(), TestCreateErrors(), TestCreateSuccess(), Middleware(), registerer, New() (+11 more)

### Community 5 - "Broker"
Cohesion: 0.09
Nodes (13): CompensationPublisher, EmailPublisher, WebhookPublisher, NewEmailPublisher(), Broker, OpenBroker(), EmailPublisher, Event (+5 more)

### Community 6 - "Payment"
Cohesion: 0.14
Nodes (12): ConflictError, NetworkStatus, PaymentRepository, TestCheckTransition(), Payment, Status, markApplied(), rowStatus() (+4 more)

### Community 7 - "saga_test.go"
Cohesion: 0.18
Nodes (16): fail(), ok(), TestCancelledCtxAbortsForward(), TestCancelledCtxStillUnwindsCompleted(), TestCompensateErrorsLoggedAndUnwindContinues(), TestDefaultSagaIDGenerated(), TestFailureLogsCarrySagaIdentity(), TestFailureStopsForwardAndUnwindsReverse() (+8 more)

### Community 8 - "Merchant"
Cohesion: 0.12
Nodes (8): Merchant, NewService(), Repository, Service, stubService, fakeLookup, notifier, webhookNotifier

### Community 9 - "NewService"
Cohesion: 0.16
Nodes (10): NewCompensationPublisher(), Publisher, AuditStore, CardNetwork, NewService(), Registry(), Publisher, intentStore (+2 more)

### Community 10 - "Consumer"
Cohesion: 0.18
Nodes (7): Consumer, Intents, Unmarshal(), headerAttempts(), NewConsumer(), Step, headerAttempts()

### Community 11 - "log/slog.Logger"
Cohesion: 0.20
Nodes (6): BySagaName(), NewLogEmailSender(), Step, Saga, Webhook, LogEmailSender

### Community 12 - "context.Context"
Cohesion: 0.26
Nodes (3): StepRecord, PaymentDatabaseRepository, stubService

### Community 13 - "Consumer"
Cohesion: 0.27
Nodes (3): Consumer, emailNotifier, EmailSender

### Community 14 - "Intent"
Cohesion: 0.27
Nodes (3): FakePublisher, Intent, PaymentDatabaseRepository

### Community 15 - "health_test.go"
Cohesion: 0.33
Nodes (6): readyHandler(), get(), okCheck(), TestHealthUnchanged(), TestReadyAllUp(), TestReadyNamesFailures()

## Knowledge Gaps
- **5 isolated node(s):** `User`, `createBody`, `createBody`, `ctxKey`, `paymentgateway`
  These have ≤1 connection - possible missing edges. (Counts symbols only; 54 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **5 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Container` connect `Build` to `worker/main.go`, `testing.T`, `AppError`, `Broker`, `log/slog.Logger`?**
  _High betweenness centrality (0.030) - this node is a cross-community bridge._
- **What connects `User`, `createBody`, `createBody` to the rest of the system?**
  _5 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `worker/main.go` be split into smaller, more focused modules?**
  _Cohesion score 0.1431989063568011 - nodes in this community are weakly interconnected._
- **Why does `Payment` connect `Payment` to `worker/main.go`, `testing.T`, `Merchant`, `NewService`, `Consumer`, `log/slog.Logger`, `context.Context`?**
  _High betweenness centrality (0.026) - this node is a cross-community bridge._
- **Should `Build` be split into smaller, more focused modules?**
  _Cohesion score 0.06168831168831169 - nodes in this community are weakly interconnected._
- **Why does `Build()` connect `Build` to `worker/main.go`, `testing.T`, `AppError`, `Broker`, `Merchant`, `NewService`?**
  _High betweenness centrality (0.021) - this node is a cross-community bridge._
- **Should `apiclient_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.10083256244218317 - nodes in this community are weakly interconnected._