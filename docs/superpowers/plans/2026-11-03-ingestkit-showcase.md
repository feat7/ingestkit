# IngestKit Engineering Showcase Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the broad experimental platform with a small, demonstrably correct event collector, reproducible measurements, and a publishable engineering article.

**Architecture:** Go/Fiber validates HTTP event envelopes and acknowledges only after Redpanda accepts them. A sequential Go consumer copies bounded batches through PostgreSQL staging into one table with a unique tenant/event key, then commits broker offsets. Invalid broker envelopes require acknowledged broker quarantine; database outages never advance source progress.

**Tech Stack:** Existing Go module (Go 1.24 minimum), Fiber v2, franz-go v1.20.3, pgx v5.7.6, PostgreSQL 18, Redpanda, Docker Compose v2, standard-library HTTP examples and benchmark driver. Add prometheus/client_golang v1.23.2 for tested counters/histograms; retain existing module versions unless a verified compatibility/security issue requires a targeted change.

**Spec:** `docs/superpowers/specs/2026-11-03-ingestkit-showcase-design.md` (approved 2026-11-03).

**Status:** Native execution in progress; tasks 1–9 verified. Demonstrations, benchmarks and writing follow. Git history and the execution ledger contain test evidence.

## Global Constraints

- Keep Go, Fiber, franz-go, pgx, PostgreSQL, Redpanda, and Docker Compose unless a verified obstacle warrants changing them.
- Initial limits: 256 KiB per event, 1 MiB per HTTP request, 100 events per HTTP batch, a five-second broker publish deadline, and 64 MiB producer buffering.
- `202 Accepted` means every record covered by that response was acknowledged by the configured broker. It does not mean PostgreSQL has stored it.
- Clients supply UUID event IDs; retries retain them. Unique storage key: `(tenant_id, event_id)`. First stored event wins; conflicting payload reuse is a caller error, without payload equality checking.
- Tenant identity comes only from the API-key mapping. Payloads are JSON objects; occurred-at timestamps are RFC 3339. Server receipt time is assigned on ingress.
- Keep PostgreSQL `fsync` and `synchronous_commit` enabled. A single-node broker is not highly available. Retention is finite and explicitly recorded.
- Commit only a contiguous successfully handled prefix in each partition. Advance invalid-record offsets only after acknowledged quarantine. Database failure is not grounds for dropping or quarantining an otherwise valid batch.
- Do not drop existing tables or volumes, reset a local database, rewrite Git history, delete remote releases, or unpublish packages.
- Use the new `ingestkit.events` schema/table and a new migration ledger. Do not reuse the old migration sequence or automatically import legacy data.
- Demonstrations, tests, and failure experiments use uniquely named Compose projects, isolated volumes/ports, and scoped cleanup. Normal stops preserve volumes.
- No unconditional zero-loss/exactly-once/production-ready claims, speculative benchmark headlines, generated SDKs, hosted product, dashboard, new destinations, or paid infrastructure.
- Publishable evidence requires raw results, source revision, environment metadata, correctness reconciliation, warm-up and at least three measured repetitions.
- Only prepare publication material. Do not push, publish, create accounts, or buy hosting as part of this plan.

## Review Focus

1. Multibyte payloads and individual events inside a legal-sized batch must obey byte limits, not character counts; trailing JSON and client-supplied tenant/receipt fields must not bypass envelope validation. Task 1 and Task 6 tests own these cases.
2. The same UUID repeated within one COPY batch must not cause merge errors or double-count inserted rows; the same ID in different tenants must remain independent. Task 2 tests own these cases.
3. Some broker records may be accepted before a publish timeout or buffer rejection; a non-success HTTP response must remain retryable with the same IDs, and callbacks arriving after that response must not block or race. Task 4 and Task 8 tests own these cases.
4. A fetch can contain both usable records and an error; a poison record or failed storage attempt must never be skipped by committing a later record from its partition. Task 5 and Task 8 tests own these cases.
5. Interrupted demos/benchmarks and concurrent runs must never delete another run's volumes, reuse a global container name, truncate pre-existing data, or leave paused resources without reporting them. Task 7, Task 10, and Task 12 tests own these cases.

## File structure and implementation decisions

The final production code is deliberately small. New packages/files can coexist with the old generated path until the runtime switch; remove the old path only after the new one works.

```text
cmd/api/main.go                   process wiring and API lifecycle
cmd/consumer/main.go              process wiring and consumer supervision
cmd/bench/main.go                 benchmark CLI, outside normal service path
internal/event/                   envelope, strict decoding, shared limits
internal/api/app.go               Fiber routes, acknowledgements, readiness
internal/api/middleware/          retained useful auth/rate-limit/error handling
internal/config/runtime.go        service environment parsing; no project generator
internal/messaging/publisher.go   bounded acknowledged publishing
internal/messaging/reader.go      poll/process/commit lifecycle
internal/messaging/process.go     ordered segment storage and quarantine
internal/messaging/quarantine.go  raw record preservation and acknowledged write
internal/persistence/events.go        transactional COPY + idempotent merge
internal/telemetry/metrics.go     private Prometheus registry and observations
internal/runenv/                  isolated Compose runner for tests/demo/bench
internal/benchmark/               seeded workloads, write-method and pipeline experiments
migrations/showcase/              single schema authority; new migration ledger
tests/integration/                explicit full-stack checks; never silently skipped
scripts/                         thin demo/test entry points
examples/checkout/               one scenario, Python/Node HTTP callers, SQL queries
benchmarks/README.md              methodology and measured results
benchmarks/results/<run-id>/      bounded JSON/CSV and environment manifest
docs/                            architecture, delivery, development, article
```

Use event-type names as data. No per-event model class, route, table, or generated file. Do not add a public read API.

Concrete settings chosen for implementation: event types and tenants are 1–128 bytes; broker publishing buffers at most 1,000 records and 64 MiB; consumer polls at most 500 records, requests 4 MiB fetches per request/partition with one concurrent fetch, and uses a 60-second rebalance timeout. Fetch byte settings are broker/client hints, not a hard RSS ceiling, particularly with compression. A storage attempt has a five-second context deadline, retry backoff starts at 100 ms and caps at two seconds, and a processing retry burst lasts at most 15 seconds. Shutdown has a ten-second overall deadline. These are documented tuning defaults, not throughput promises.

Use `kgo.MaxBufferedBytes(64<<20)`, `kgo.MaxBufferedRecords(1000)`, `kgo.RequiredAcks(kgo.AllISRAcks())`, and `TryProduce`. A per-call result channel has capacity equal to the number of records; callbacks never depend on an active HTTP handler. `Publish` selects on its own five-second deadline rather than waiting unconditionally for callbacks: franz-go idempotent in-flight delivery can outlast cancellation. A timeout is ambiguous, not proof of non-delivery. Retain the library's idempotence; do not disable it to force a convenient timeout.

The reader uses `DisableAutoCommit`, `BlockRebalanceOnPoll`, `PollRecords(ctx, 500)`, explicit successful-prefix `CommitRecords`, and `CloseAllowingRebalance`. Failed processing never resumes polling past the failed batch. After an exhausted retry burst or commit error, close the old reader and rejoin from committed offsets with cancellable backoff; the service supervisor remains unready until dependencies recover. This bounds time spent blocking a rebalance while preserving source records.

Topic defaults: `ingestkit.events` (three partitions, 512 KiB maximum message/batch setting), `ingestkit.dlq` (one partition, 2 MiB maximum), replication factor one for the explicitly local stack, and seven-day retention (`604800000` ms, configurable). Validate the exact broker setting/record-overhead semantics in the topic initialization integration test. The DLQ allowance accommodates base64 preservation of malformed source bytes. Disable automatic topic creation in service clients; topic initialization must succeed before services become ready. Partition keys combine tenant and event ID, so tenant-wide event order is not promised.

Images verified from upstream during planning: `postgres:18.6-alpine`, `docker.redpanda.com/redpandadata/redpanda:v26.2.6`, and `migrate/migrate:v4.20.1`. Confirm arm64/amd64 manifests before implementation uses them, and record pulled digests. Pin the Go build/runtime image patches during Task 7 after checking availability rather than retaining floating/EOL images. PostgreSQL 18 mounts data at `/var/lib/postgresql`, not the old `/var/lib/postgresql/data` target.

Migration runner points only at `migrations/showcase` and uses `search_path=public&x-migrations-table=ingestkit_schema_migrations`. Inspect available legacy ledger names without modifying them. Migration failures must exit nonzero; never append `|| true` or `|| exit 0`.

Primary references used for these choices:

- [franz-go publishing and limits](https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo#Client.TryProduce); exact v1.20.3 option/cancellation behaviour also checked in the local module source.
- [PostgreSQL official image and volume location](https://hub.docker.com/_/postgres).
- [Redpanda quickstart and pinned image](https://docs.redpanda.com/streaming/current/get-started/quick-start/).
- [migrate release](https://github.com/golang-migrate/migrate/releases/tag/v4.20.1) and [custom migration ledger](https://github.com/golang-migrate/migrate/blob/master/database/postgres/README.md).
- [Prometheus client release](https://github.com/prometheus/client_golang/releases/tag/v1.23.2).
- [Compose project isolation](https://docs.docker.com/compose/how-tos/project-name/).

## Execution and verification conventions

- Work in this current checkout unless the user chooses isolation at execution time. Put implementation on a local `refactor/engineering-showcase` branch before the first code task; keep the existing design commit. Never push automatically.
- Baseline `go test -race ./...` passed at `d81ef9c`. Re-run it before code changes, then run each task's targeted checks. Use a writable Go cache (`GOCACHE=/private/tmp/ingestkit-audit-go-cache` in this environment). Do not repeatedly rerun an unchanged expensive suite.
- Tests marked `//go:build integration` require real dependencies and fail if their connection settings are missing. Normal unit tests remain service-independent.
- For each code task: write the specified behavioural test, confirm the expected failure, implement the smallest change, run targeted tests, then relevant regression checks and commit only that task's files. Do not write tests that merely inspect source text for an implementation detail.
- The commit step includes `git diff --check` and a scoped `git add`/`git commit`. Record the actual commit and completed checkboxes in this plan. Exact commits may combine tightly coupled file edits but must not conceal unrelated work.
- Full-stack/Docker access is a prerequisite to phase 2 verification, not a final-day discovery. Request the sandbox capability if needed; do not touch an existing stack to make tests pass.

---

### Task 1: Define and test the event contract

**Files:** Create `internal/event/event.go`, `internal/event/decode.go`, `internal/event/event_test.go`, `examples/checkout/events.json`; add the new contract to `docs/delivery.md` while leaving legacy entry points operational.

**Interfaces:** Produce `Input{EventID uuid.UUID, EventType string, OccurredAt time.Time, Payload json.RawMessage}`, `Envelope{Input, TenantID string, ReceivedAt time.Time}`, `DecodeInput(raw []byte) (Input, error)`, `DecodeBatch(raw []byte) ([]Input, error)`, `Accept(input Input, tenant string, receivedAt time.Time) (Envelope, error)`, `EncodeEnvelope(envelope Envelope) ([]byte,error)`, and `DecodeEnvelope(raw []byte) (Envelope, error)`. Export `MaxEventBytes=256<<10`, `MaxEnvelopeBytes=MaxEventBytes+4096`, `MaxRequestBytes=1<<20`, `MaxBatchEvents=100`, and a typed validation error distinguishing too-large from malformed input. JSON field names match the spec. Decode exactly one JSON document and disallow unknown envelope-level fields; payload contents remain application-owned. Broker envelopes get the explicit metadata allowance so a legal maximum-sized HTTP event is not rejected after adding server fields; encode without HTML escaping to avoid expanding otherwise valid payloads beyond that allowance.

- [x] **1. Write failing tests:** `TestDecodeValidEvent`, `TestDecodeRejectsInvalidEnvelope` (missing/nil UUID, empty/129-byte event type, bad timestamp, null/array/scalar payload, trailing JSON, tenant/received-at injection), `TestEventByteLimits`, `TestBatchLimitsAndValidation`, `TestAcceptUsesServerIdentity`, and `TestDecodeBrokerEnvelope`. Representative assertions:
  ```go
  require.Error(t, errForPayload(`{"payload":null}`))
  require.Len(t, decodedInputsForExactly100ValidEvents, 100)
  require.Error(t, errFor101ValidEvents)
  require.Equal(t, "tenant-a", accepted.TenantID)
  require.Equal(t, receiptTime, accepted.ReceivedAt)
  ```
  Build UTF-8 fixtures that cross 256 KiB in bytes while remaining under the character count; validate per-event byte limits inside batches and the 1 MiB aggregate limit. `TestEnvelopeEncodingAtInputLimit` round-trips a valid 256 KiB input containing many `<>&` characters through encode/decode with metadata and verifies the broker-size bound. Reject zero tenant/receipt metadata on broker envelopes. Fix fixture times/IDs; no sleeps.
- [x] **2. Run red:** `go test ./internal/event -count=1`; expect missing types/functions or failing contract assertions.
- [x] **3. Implement** the defined types/decoders/encoder and three-event checkout fixture. Canonicalize UUIDs via `uuid.UUID`; preserve payload contents (insignificant JSON whitespace may be normalized); assign one receipt time at ingress. Use `json.Encoder.SetEscapeHTML(false)` for broker encoding. Do not introduce a schema DSL or validate application-specific payload fields.
- [x] **4. Run green:** `go test -race ./internal/event ./internal/api/middleware`; expect all tests pass and legacy build unaffected.
- [x] **5. Commit:** `feat: define explicit ingestion event contract`.

### Task 2: Build isolated migration and idempotent storage verification

**Files:** Create `migrations/showcase/000001_events.up.sql`, `internal/persistence/events.go`, `internal/persistence/events_integration_test.go`, `tests/compose.yml` (temporary infrastructure-only test base), and `scripts/test-storage.sh`. Modify existing storage files only if a name conflict requires it; retain legacy paths until Task 9.

**Interfaces:** Consume Task 1 envelopes. Produce `NewEventStore(pool *pgxpool.Pool) *EventStore`, `WriteBatch(ctx context.Context, events []event.Envelope) (WriteResult, error)`, `WriteResult{Inserted int64, Duplicates int64}`, and `Ping(ctx context.Context) error`. The caller owns the pool. Expose no public database reset method.

- [x] **1. Write failing real-PostgreSQL tests:** `TestWriteBatchPreservesEnvelope`, `TestWriteBatchDeduplicatesWithinAndAcrossBatches`, `TestWriteBatchScopesIDsByTenant`, `TestWriteBatchRollsBackOnFailure`, `TestWriteBatchCancellation`, and `TestMigrationPreservesLegacyTablesAndLedger`. Assert returned IDs/payload/time round-trip, `Inserted=1`/`Duplicates=1` for two copies of one ID, two rows for one ID in two tenants, and zero newly visible rows after a cancelled/failed transaction. Create legacy sentinel table/ledger rows and assert unchanged after migration.
- [x] **2. Run red:** start only a uniquely named test PostgreSQL/migration stack with dynamic loopback ports; run `go test -tags=integration ./internal/persistence -count=1`. Missing implementation/schema must fail; unavailable DB must fail rather than skip. Confirm image manifests and daemon capability before proceeding.
- [x] **3. Implement** the migration and transactional temporary staging (`ON COMMIT DROP`), `CopyFrom`, and `INSERT ... ON CONFLICT DO NOTHING`. Primary key is `(tenant_id,event_id)`; `payload` is constrained to an object. Count inserts from command results and duplicates from submitted count minus inserts. Use `persisted_at DEFAULT clock_timestamp()` as a database insertion timestamp, explicitly not a measurement of physical commit completion. Keep transaction/commit errors observable and roll back on every failure. Record inspected legacy ledger names without modifying them.
- [x] **4. Run green:** `scripts/test-storage.sh`; then `go test ./...` and `git diff --check`. A failed migration must fail the runner. Keep all old tables/volumes intact. Record actual image digests.
- [x] **5. Commit:** `feat: persist events with transactional copy and deduplication`.

### Task 3: Add bounded, accurate service telemetry

**Files:** Create `internal/telemetry/metrics.go`, `internal/telemetry/metrics_test.go`; modify `go.mod`, `go.sum` for the pinned Prometheus client.

**Interfaces:** Produce `New() *Metrics`, `Handler() http.Handler`, `ObserveHTTP(route, status string, duration time.Duration)`, `ObservePublish(outcome string, duration time.Duration)`, `ObserveWrite(result storage.WriteResult, received []time.Time, duration time.Duration)`, `ObserveRetry()`, and `ObserveQuarantine(outcome string)`. A private registry belongs to each instance; do not mutate the global registry. Runtime readiness is handled separately, not inferred from event counts.

- [x] **1. Write failing tests:** `TestMetricsIndependentRegistries`, `TestMetricsCountersAndHistogramBuckets`, `TestMetricsHaveBoundedLabels`, `TestReceiptLatencyObservedAfterCommit`. Assert exact counter increments for known operations and histogram sample counts, no invented events/sec gauge, and no labels populated from tenant IDs, event IDs, or arbitrary event types. Use controlled receipt times and successful store completion, not event occurred-at time.
- [x] **2. Run red:** `go test ./internal/telemetry -count=1`; expect missing package/functions.
- [x] **3. Implement** Prometheus counters/histograms with fixed route/outcome/status classes. Ignore/clamp negative measured receipt durations defensively and document the same-host clock assumption. Do not add dashboards or a Prometheus-server container.
- [x] **4. Run green:** `go test -race ./internal/telemetry`; `go test ./...`. Verify the pinned dependency supports the module's Go version; do not broadly upgrade dependencies.
- [x] **5. Commit:** `feat: expose reliable ingestion counters and latency histograms`.

### Task 4: Implement acknowledged publishing with real backpressure

**Files:** Create `internal/messaging/publisher.go`, `internal/messaging/publisher_test.go`, `internal/messaging/publisher_integration_test.go`, and `scripts/test-messaging.sh`; extend the temporary `tests/compose.yml` with the isolated broker and explicit topic initialization. Keep old `Producer` so the old API still builds.

**Interfaces:** Consume `event.Envelope`, optional `*telemetry.Metrics`. Produce `PublisherConfig{Brokers []string, Topic string, MaxBufferedBytes int, MaxBufferedRecords int, PublishTimeout time.Duration}`, `NewPublisher(config PublisherConfig, metrics *telemetry.Metrics) (*Publisher,error)`, `Publish(ctx context.Context, events []event.Envelope) error`, `Ping(ctx context.Context) error`, `Flush(ctx context.Context) error`, and `Close()`. Define errors `ErrSaturated` and `ErrUnavailable` for `errors.Is`. The public behaviour does not expose kgo records.

- [x] **1. Write failing tests:** `TestPublishWaitsForEveryAck`, `TestPublishFailsForAnyRecordError`, `TestPublishDeadlineDoesNotWaitForLateCallback`, `TestLateCallbackIsNonblocking`, `TestPublishReportsBufferSaturation`, `TestPublishOwnsRecordBytes`, and `TestPublisherRejectsMissingTopic`. Fake only the produce callback boundary. Assert no success before all acknowledgements, return by a short test deadline even if callbacks have not completed, safe callback completion afterward, and correct error classification for a partially queued batch. Race-test concurrent requests. Add tagged `TestPublisherRoundTrip`, `TestPublisherBrokerUnavailable`, `TestPublisherNoAutoTopicCreation`, and `TestPublisherBufferLimitIntegration`; assert acknowledged IDs are readable from the real initialized topic, a missing topic remains absent, and unavailable/saturated publishing fails within the caller deadline.
- [x] **2. Run red:** `go test ./internal/messaging -run 'TestPublish|TestLate|TestPublisher' -count=1`.
- [x] **3. Implement** the fixed settings above with `TryProduce`, a bounded completion channel, caller-deadline selection, and copied/owned record bytes. Publish canonical broker envelopes and key by tenant plus UUID. Treat success as all callbacks acknowledged. `Ping` verifies both broker connectivity and the configured topic's metadata; initialization is not proof of readiness.
- [x] **4. Run green:** targeted race tests plus `TestPublisherRoundTrip`, `TestPublisherBrokerUnavailable`, `TestPublisherNoAutoTopicCreation`, and actual small-buffer saturation against the isolated broker. Assert durable acknowledged records can be consumed after closing the publisher. Do not weaken idempotence or limits to pass a timing check.
- [x] **5. Commit:** `feat: acknowledge ingestion only after bounded broker publishing`.

### Task 5: Process ordered source records without unsafe offset advancement

**Files:** Create `internal/messaging/reader.go`, `process.go`, `quarantine.go`, `reader_test.go`, `process_test.go`, `quarantine_test.go`, and `reader_integration_test.go`. Retain the old `Consumer` until runtime replacement.

**Interfaces:** Consume `EventStore.WriteBatch`, `Metrics`, and event decoding. Produce `ReaderConfig{Brokers []string, Topic string, DLQTopic string, GroupID string, BatchSize int, AttemptTimeout time.Duration, RetryWindow time.Duration, BackoffMin time.Duration, BackoffMax time.Duration}`, `NewReader(config ReaderConfig, store BatchStore, metrics *telemetry.Metrics) (*Reader,error)`, `Run(ctx context.Context) error`, `Ping(ctx context.Context) error`, and `Close()`. Define `BatchStore` as `WriteBatch(context.Context, []event.Envelope) (storage.WriteResult,error)`; return `ErrRetryExhausted` for a recoverable failed processing burst. Internal `Quarantine` preserves source topic, partition, offset, key, headers, original raw bytes, and a bounded error description; `Write(ctx, record *kgo.Record, cause error) error` waits for broker acknowledgement.

- [x] **1. Write failing tests:** `TestProcessCommitsOnlyAfterStorage`, `TestProcessDoesNotCommitAfterStorageFailure`, `TestProcessCannotCommitPastPoisonRecord`, `TestQuarantineFailureDoesNotAdvanceSource`, `TestRetryCancellation`, `TestRetryExhaustionStopsPolling`, `TestCommitFailureStopsPolling`, `TestMixedFetchErrorDoesNotSkipValidRecords`, and `TestRebalanceAlwaysReleased`. Assert call order with an operation trace:
  ```go
  require.Equal(t, []string{"store:0", "commit:0", "dlq:1", "commit:1", "store:2", "commit:2"}, trace)
  require.NotContains(t, traceAfterDLQFailure, "commit:2")
  require.Equal(t, 1, pollsBeforeRetryExhaustion)
  ```
  Test separate partitions, nonconsecutive physical offsets (compaction/gaps), cancellation during backoff, and bounded malformed-record error text. Add tagged `TestReaderStoreRoundTrip` using the Task 2/4 real dependencies; assert submitted IDs reach the new table before their offsets commit, then cancel and verify prompt termination.
- [x] **2. Run red:** `go test ./internal/messaging -run 'TestProcess|TestQuarantine|TestRetry|TestCommit|TestMixed|TestRebalance' -count=1`.
- [x] **3. Implement** ordered contiguous valid segments per partition; flush a segment before quarantining an invalid record. Commit only completed segments/acknowledged quarantines. Handle valid records in a fetch even when another partition reports an error; never silently discard them. Unexpected DB errors retry, then return without polling further. On all exits release blocked rebalances and leave unfinished work uncommitted. Use bounded fetch defaults and explicit source progress; no mark-and-autocommit path.
- [x] **4. Run green:** targeted race tests, `go test ./...`, and real broker/DB happy path using a new group and known offsets. Confirm shutdown cancellation is prompt and does not falsely increment stored/quarantined counters.
- [x] **5. Commit:** `feat: preserve consumer progress through storage and quarantine failures`.

### Task 6: Expose the small HTTP contract and dependency-aware readiness

**Files:** Create `internal/api/app.go`, `internal/api/ingest.go`, `internal/api/app_test.go`. Modify useful middleware only for verified lifecycle/configuration needs; leave `cmd/api` on its old wiring until Task 7.

**Interfaces:** Consume Task 1, Task 3 and `EventPublisher{Publish(context.Context, []event.Envelope) error; Ping(context.Context) error}`. Produce `Config{APIKeys map[string]string, RateLimitRPS int, PublishTimeout time.Duration}`, `NewApp(config Config, publisher EventPublisher, metrics *telemetry.Metrics) (*fiber.App, func(), error)`; the cleanup function stops owned rate-limiter resources and is safe once after shutdown. Routes: `/v1/events`, `/v1/events/batch`, `/health`, `/ready`, `/metrics`.

- [x] **1. Write failing tests:** `TestIngestAcknowledgementBoundary`, `TestIngestSingleAndBatchContract`, `TestInvalidBatchProducesNothing`, `TestIngestRejectsIdentityInjection`, `TestIngestEnforcesBodyAndEventLimits`, `TestIngestBrokerTimeoutAndPartialFailure`, `TestAuthAndRateLimit`, `TestReadyChecksPublisher`, `TestMetricsRoute`. Assert successful response shape `{"status":"accepted","event_ids":[...],"request_id":"..."}`, `202` only after fake acknowledgement, all-or-error input validation, exact statuses `400/401/413/429/503`, and unchanged caller IDs on retry. Unknown old `/:type`/schema routes return `404`. Errors contain no full payloads or credentials.
- [x] **2. Run red:** `go test ./internal/api -count=1`.
- [x] **3. Implement** Fiber handlers using strict raw-message decoding and owned request bytes. Bind tenant from existing auth middleware; assign one receipt timestamp per request. Use a standard independent context with a deadline, never retain Fiber request context/body in asynchronous callbacks. Adapt the private Prometheus handler with Fiber's bundled `middleware/adaptor`. Liveness is local; readiness checks real dependencies under a short timeout.
- [x] **4. Run green:** `go test -race ./internal/api/... ./internal/event ./internal/messaging`; preserve useful middleware tests, and prove the rate limiter's cleanup exits.
- [x] **5. Commit:** `feat: expose validated broker-acknowledged event endpoints`.

### Task 7: Wire the new services and run the complete isolated stack

**Files:** Replace `cmd/api/main.go`, `cmd/consumer/main.go`; create `cmd/api/main_test.go`, `cmd/consumer/main_test.go`, `internal/config/runtime.go`, `runtime_test.go`, `internal/runenv/compose.go`, `compose_test.go`, `scripts/test-integration.sh`, `tests/integration/pipeline_test.go`; rewrite Dockerfiles, Compose, `.env.example`, `.dockerignore`, and relevant Make targets. Change `tests/compose.yml` from the temporary infrastructure base into a test override of the single root Compose definition. Remove legacy `tests/integration/e2e_test.go` only when the new success test is running.

**Interfaces:** Produce `config.LoadAPIEnv(getenv func(string) string) (APIEnv,error)` and `LoadConsumerEnv(getenv func(string) string) (ConsumerEnv,error)` with typed fields for the configurations above; required settings are explicit and validated. Use `API_KEYS` as a JSON key-to-tenant map, `DATABASE_URL`, `BROKERS`, `EVENT_TOPIC`, `DLQ_TOPIC`, `CONSUMER_GROUP`, and service listen addresses. No default credentials in service code; the local Compose file supplies marked local-only values. Produce `runenv.Start(ctx context.Context, options Options) (*Stack,error)`, `Stack.APIURL`, `Stack.DatabaseURL`, `Stack.Brokers`, `Stack.Project`, `Stack.Stop(ctx) error`, `Stack.Destroy(ctx) error`, `Stack.SetServiceRunning(ctx, service string, running bool) error`, and `Stack.CollectEvidence(ctx, directory string) error`. `Options` includes purpose and output directory; the runner creates its own validated project name and discovers loopback ephemeral ports.

Both command packages expose an internal `run(ctx context.Context) error` so `main` only handles signals, logs the outcome, and chooses exit status. The runner lets Docker allocate API/database ports. The broker's advertised external address must match its host port: reserve a free loopback port before startup, inject that port into both the mapping and advertised listener, then release the reservation just before Compose binds. Retry a port-bind race at most three times using only newly owned resources. A random mapping with a fixed advertised `localhost:19092` is incorrect and must fail a host-client integration test.

- [x] **1. Write failing tests:** `TestRuntimeConfigValidation`, `TestRunenvScopesEveryComposeCommand`, `TestRunenvRejectsUnownedCleanup`, `TestRunenvDiscoversDynamicPorts`, `TestAPIShutdownStopsAdmissionBeforeFlush`, `TestConsumerSupervisorRejoinsAfterFailure`, `TestShutdownTimeoutIsFailure`, and tagged `TestPipelineHappyPath`. Use a command-execution seam for runner safety tests and real services for the pipeline test. Assert `-p` is always present, no static container/global volume names, no destructive cleanup of a supplied arbitrary project, receipt IDs round-trip, and fresh-stack migrations run automatically.
- [x] **2. Run red:** targeted unit tests and `scripts/test-integration.sh -run TestPipelineHappyPath`; distinguish missing wiring from daemon/image access failures and resolve the latter now.
- [x] **3. Implement** runtime wiring, readiness, and a consumer supervision loop: close/recreate a failed reader after cancellable backoff, keep readiness false while dependencies are down, and never poll the old client after a processing failure. API shutdown stops admission, drains requests, flushes, then closes; deadline failures exit nonzero. Avoid allowing an unbounded library close to conceal the ten-second shutdown deadline. Root Compose has no `container_name`, fixed volume names, or hidden migration errors; removes Console from default services; initializes explicit topics and mounts PostgreSQL 18 data correctly. Test/demo runner uses the root definition plus override and scoped labels/ports. Preserve volumes on `make down`.
- [x] **4. Run green:** `go test -race ./...`, `go build ./cmd/api ./cmd/consumer`, `docker compose config -q`, and the real success integration test. Start two independent test stacks concurrently and verify distinct ports/volumes. Collect Docker/image metadata without exposing secrets. Confirm failure before/after migration does not affect legacy tables.
- [x] **5. Commit:** `refactor: run the focused collector without code generation`.

### Task 8: Prove the failure and recovery contract against real services

**Files:** Create `tests/integration/recovery_test.go`, `quarantine_test.go`, `lifecycle_test.go`, `helpers_test.go`; extend publisher/reader integration checks and `internal/runenv` only for required failure controls. Add a normal-code test seam at the database-commit/offset-commit boundary if needed; never expose an unauthenticated production HTTP failpoint.

**Interfaces:** Consume the complete stack and direct kgo/pgx clients. Helpers `waitForIDs(ctx, pool, tenant, ids) error`, `committedOffset(ctx, brokers, group, topic, partition) (int64,error)`, and `assertNoUnexpectedRows(...)` reconcile concrete sets, not just row totals. All test deadlines are bounded; readiness polling replaces fixed sleeps.

- [x] **1. Write failing tests:** `TestDatabaseOutageRecovery`, `TestCrashBetweenDatabaseAndOffsetCommit`, `TestMalformedRecordQuarantine`, `TestFailedQuarantineCannotSkipSource`, `TestBrokerOutageAndStableIDRetry`, `TestConsumerRebalanceRecovery`, `TestGracefulAndForcedShutdown`, `TestIntegrationMissingConfigurationFails`. Force the relevant boundary rather than hoping a race occurs. For failure-before-offset commit, run the real reader/store in a test helper process whose store wrapper signals after commit and blocks before returning, then terminate it and restart the normal consumer. Assert acknowledged IDs equal stored IDs, one row per tenant/ID, exact committed-prefix behaviour, successful quarantine preserves raw bytes/source metadata, and restored service readiness.
- [x] **2. Run red:** run each new test against isolated services and confirm it detects the injected failure if the relevant safety condition is deliberately broken in a temporary local patch; restore that patch immediately. Never weaken the test to accept data loss.
- [x] **3. Implement/fix** only proven gaps in the new core. Broker quarantine failure is induced with a missing/unavailable DLQ topic and automatic topic creation disabled; restore it and prove eventual progress. Database outage remains valid-event retry, not quarantine. Record tested outage durations and retention settings.
- [x] **4. Run green:** `scripts/test-integration.sh` plus `go test -race ./...`. Require zero skipped safety tests and no unexplained IDs. Save failure diagnostics on test failure; test cleanup unpauses/restores its owned services before destroying its own stack.
- [x] **5. Commit:** `test: prove ingestion recovery and safe source progress`.

### Task 9: Remove the superseded platform and consolidate CI/documentation

**Files:** Delete `cmd/cli/`, `internal/schema/`, `schema/`, `generated/`, `packages/`, old `internal/validation/`, legacy `internal/messaging/producer.go`/`consumer.go` and obsolete tests, `internal/storage/dlq.go`/`partitions.go`, CLI-only portions of `internal/config/config.go`, and old examples/loadtest/benchmark paths. Delete the obsolete guides/scripts/migration paths enumerated in the spec. Rewrite `.github/workflows/ci.yml`, retire `.github/workflows/release.yml` package publication, simplify Makefile/config/ignores, and replace README/CONTRIBUTING with concise truthful startup instructions. Preserve LICENSE/CODE_OF_CONDUCT, approved design/plan, new migrations and new runtime files.

**Interfaces:** The only public commands are `make help`, `make up`, `make down`, `make logs`, `make test`, `make test-integration`, later demo and bench targets, and documented direct Go builds. `make up` waits for healthy services and fails on initialization problems. CI runs unit/race checks, `go vet`, builds, and real full-stack integration. Later tasks add demo smoke checks. No compiler, registry installer, generated file, or existing external release is needed.

- [x] **1. Establish pre-cut proof:** `go test -race ./...`, full integration suite, and fresh image builds must pass on the replacement path. Record the new packages' import graph; confirm no production imports use `generated`, schema compiler, CLI project detection, or old messaging types.
- [x] **2. Remove only obsolete paths** from the approved cut list. Remove old migration files from the repository only after the new ledger/path preservation test passes; never remove tables/volumes/remote packages. Remove repeated unsupported performance and zero-loss copy while replacing the public README in the same commit, so it never points at removed commands.
- [x] **3. Implement CI/Make/docs changes**, run `go mod tidy`, and remove dependencies solely used by deleted code (`lib/pq`, schema YAML, etc., when no remaining imports need them). Pin CI action versions and image patches after checking official compatibility; avoid unsolicited toolchain upgrades. Keep the spec/plan as historical internal records, excluded from public stale-command checks.
- [x] **4. Verify:** unit/race suite, `go vet ./...`, build all commands, full integration, Docker build without `generated/`, fresh-start README commands, and `rg` for removed imports/commands throughout maintained code/docs/workflows. Review the diff as a deletion manifest and compare with every keep/remove row in the spec; account for each path. No source-text unit tests for this reversible cleanup.
- [x] **5. Commit:** `refactor: remove compiler distributions and obsolete platform scaffolding`.

### Task 10: Provide one deterministic demo and two tiny reusable callers

**Files:** Create `scripts/demo.sh`, `scripts/demo-recovery.sh`, `examples/checkout/send.py`, `send.mjs`, `queries.sql`, `README.md`, and `internal/runenv/demo.go`/`demo_test.go` if Go is needed for portable readiness/reconciliation. Extend Makefile and CI; remove temporary orchestration helpers replaced by the shared runner.

**Interfaces:** `make demo` and `make demo-recovery` create their own stack/run directory, print its location and accepted IDs, produce a reconciliation manifest, and exit nonzero on missing/unexpected IDs. `send.py` uses Python standard-library urllib; `send.mjs` uses Node's standard HTTP/fetch facilities. Both accept API URL/key via environment, explicit stable event IDs, single/batch requests, and bounded stable-ID retries for `429/503`/ambiguous network errors. Neither generates code or installs an IngestKit package.

- [ ] **1. Write failing workflow checks:** `TestCheckoutDemoReconciles`, `TestRecoveryDemoReconciles`, `TestDemoInterruptCollectsEvidenceAndScopesCleanup`, and tagged `TestPythonAndNodeCallersAgree`. Assert three event types yield the expected small SQL funnel, every accepted ID is present once, recovery sends events while PostgreSQL is actually unavailable, and an intentionally unreachable API produces a nonzero caller exit. Verify fixed IDs remain stable across retries and same test fixture works through curl/Python/Node.
- [ ] **2. Run red:** run the new scripts/tests against isolated resources; expected failure is missing implementation or missing reconciliation, not a silent connection failure.
- [ ] **3. Implement** thin scripts over the shared runner and scenario fixture. Keep normal demo and outage demo separate. Capture acknowledgements, final ID sets, configuration without secrets, broker lag snapshots, and cleanup status. Normal demo stopping preserves its volumes and reports a scoped optional cleanup command; temporary CI runs can destroy their own resources. Add a small demo smoke job to CI, without expensive load tests.
- [ ] **4. Run green:** both demos from a fresh checkout-equivalent working directory, Python/Node contract check, and concurrent-run/interrupt checks. Verify failed runs leave an explicit evidence path and no undisclosed paused resources.
- [ ] **5. Commit:** `feat: add reproducible checkout and outage recovery demonstrations`.

### Task 11: Build a fair database write-method experiment

**Files:** Create `cmd/bench/main.go`, `internal/benchmark/workload.go`, `workload_test.go`, `database.go`, `database_integration_test.go`, `results.go`, `results_test.go`, and `benchmarks/README.md`; add `make bench-db`.

**Interfaces:** Produce `Workload(config WorkloadConfig) ([]event.Envelope,error)` using seed, count, payload size and duplicate fraction; `RunDatabase(ctx context.Context, config DatabaseConfig) (DatabaseReport,error)`; and `WriteReport(directory string, report any) error`. Report includes methods `insert`, `batch-insert`, `copy-merge`, every repetition, submitted/inserted/duplicate counts, elapsed interval, correctness, and environment. The production store's COPY implementation is used for the copy-merge case, not a separate faster approximation.

- [ ] **1. Write failing tests:** `TestWorkloadSeedReproducesIDsAndBytes`, `TestAllWriteMethodsProduceSameIDSet`, `TestDatabaseReportRejectsIncorrectRun`, `TestDuplicateWorkloadHasMatchingSemantics`, `TestTimingExcludesSetup`, `TestResultsContainEveryRepetition`. Assert exact equal ID/payload sets across all methods, duplicate counts, at least three measured samples, and rejection of zero/negative counts or batch sizes. Verify partial insertion errors cannot become a successful zero-duration speed result.
- [ ] **2. Run red:** `go test ./internal/benchmark`; tagged DB tests must fail before method implementations exist.
- [ ] **3. Implement** comparable single INSERT, parameterized batch INSERT, and COPY-to-staging/merge using the same table/indexes/durability/duplicate policy. Precompute workload outside timing, warm up each method, rotate method order across repetitions, and reset only the owned run's dataset outside timing. No arbitrary unvalidated SQL identifiers or global TRUNCATE. Keep sample defaults bounded (10,000 events/method, three measured repetitions) and configurable for later serious measurements.
- [ ] **4. Run green:** unit plus real DB comparison; validate report schema, per-run raw JSON/CSV, median elapsed/throughput calculations, and correctness failure exit status. Do not put a performance number into README yet.
- [ ] **5. Commit:** `feat: benchmark database write methods with equal correctness semantics`.

### Task 12: Measure the whole pipeline and publish reproducible local evidence

**Files:** Create `internal/benchmark/pipeline.go`, `pipeline_test.go`, `pipeline_integration_test.go`, `manifest.go`, `manifest_test.go`, extend `cmd/bench/main.go`, Makefile and `benchmarks/README.md`, and add bounded results under `benchmarks/results/<run-id>/` after the harness is committed.

**Interfaces:** Produce `RunPipeline(ctx context.Context, config PipelineConfig) (PipelineReport,error)` for steady/burst runs. Configuration specifies API URL, scoped key/tenant, seed, attempted rate/concurrency, duration, warm-up, payload bytes, HTTP batch size, repetitions and drain deadline. Reuse `runenv.Stack` and shared deterministic workloads. Record accepted IDs separately from ambiguous/failed requests; reconcile accepted IDs to persisted IDs and report late/ambiguous deliveries distinctly.

- [ ] **1. Write failing tests:** `TestPipelineAcceptanceIsNotPersistenceRate`, `TestPipelineReconcilesSetsNotOnlyCounts`, `TestPipelineReportsLoadGeneratorLimits`, `TestLatencyPercentilesUseDefinedSamples`, `TestManifestRedactsSecrets`, `TestBenchmarkCleanupPreservesUnownedData`, and `TestPipelineDrainDeadlineFails`. Assert an HTTP-accepted-but-unpersisted event invalidates completion; duplicates and unexpected IDs are counted; duration includes measured steady interval and separately reported drain; no API key/credentialled DSN is saved. Percentile tests use a fixed sample vector with known expected results.
- [ ] **2. Run red:** targeted unit/tagged workflow tests; missing storage evidence must fail even if all HTTP responses are `202`.
- [ ] **3. Implement** one Go load driver, not another HTTP server mode or framework. Measure actual generated load (including misses), HTTP/broker-ack latency, persisted unique events/sec, receipt-to-visible-storage latency and polling resolution, backlog/drain, failures and resource snapshots from `docker stats`. `persisted_at` remains an insertion timestamp; compute confirmed persistence latency after DB commit via collector telemetry or a clearly labelled observation bound, never claim it measures physical commit from the SQL default. Record shared-host clock/resource contention. Keep recovery results separate from normal throughput.
- [ ] **4. Verify and commit the harness:** unit/race suite, small real pipeline correctness run, report checks; commit `feat: measure pipeline persistence latency throughput and backlog`. Run meaningful measurements only from this clean fixed code revision: warm-up plus three measured repetitions for each selected steady/burst configuration. Write source SHA, image digests, Go/OS/CPU/memory/disk/runtime versions, all settings (including `SHOW fsync`/`SHOW synchronous_commit`, partitions/replication/retention), local/shared-generator caveats, and all raw results. Mark resource/client saturation as a measurement limit, not collector capacity. Investigate invalid runs instead of hiding them.
- [ ] **5. Commit verified evidence:** `bench: record reproducible local ingestion experiments`. Keep captures bounded; large external artifacts require location/checksum. If the environment prevents a meaningful benchmark, leave the measurement task incomplete with the actual blocker and no headline claim.

### Task 13: Finish the public repository and engineering article

**Files:** Finalize `README.md`, `CONTRIBUTING.md`, `docs/architecture.md`, `docs/delivery.md`, `docs/development.md`, `docs/article.md`, `benchmarks/README.md`, and a compact evidence figure/capture under `docs/assets/` if useful. Save submission/resume copy in `docs/superpowers/publication/2026-11-03-launch-drafts.md`, outside the public article's prose. Update this plan's progress and spec verification notes.

**Interfaces:** README gives a short problem statement, present status, architecture figure, one start/demo path, one event example, verified results with environment links, boundaries/limitations, and deeper links. Article references the exact code revision/results. Maintain a factual one-paragraph project description/resume bullet grounded in performed work, without invented adoption or production scale. No new website/dashboard/platform is required.

- [ ] **1. Build a claim-to-evidence checklist:** map every number and delivery claim to a result/test/code location. Enumerate every public setup/demo/benchmark command and internal Markdown link. This is editorial verification, not a test that mirrors Markdown text.
- [ ] **2. Draft** the article around the problem, baseline comparison, batching choice, broker acknowledgement boundary, outage/replay experiments, limitations, and reproduction. Distinguish newly performed experiments from historical development; explain why Kafka/Redpanda helps during outages and why small workloads may not justify it. Include measured duplicate-handling overhead and the cost of the selected simpler generic storage if observed. Do not fabricate a progression of performance gains.
- [ ] **3. Verify** all documented commands in a clean isolated stack, check maintained links/paths, inspect rendered README/diagrams/article layout, and confirm no root onboarding link points to internal planning records. Review the approved deletion manifest again for leftovers. Prepare a concise regular HN article submission and a separate potential Show HN description only when the runnable artifact is ready; save drafts locally without posting them.
- [ ] **4. Run final checks:** unit/race suite, `go vet ./...`, all command builds, full integration/recovery suite, demo smoke, recorded benchmark validity, and `git diff --check`. Request a fresh whole-branch code review using superpowers' review workflow, resolve actionable issues, and rerun only affected checks plus the necessary final regression set. Review the article for technical consistency as well as code correctness.
- [ ] **5. Commit:** `docs: present the ingestion experiments and publishable engineering story`. Report what works, actual measured limits, remaining gaps, and links to the article/results. Prepare the branch for the user's chosen integration; do not merge, push, publish or claim public popularity automatically.

## Self-review and coverage

- Contract/limits/auth/readiness: Tasks 1, 4, 6, 7; verified identity and timeout ambiguity are explicit tests.
- COPY, duplicate policy, migration/data preservation: Task 2; repeated IDs inside one batch and across tenants are covered.
- Source progress, quarantine, retry, cancellation, rebalance, shutdown: Tasks 5, 7, 8; no manual test relying on a lucky crash window.
- Observability and receipt/persistence timing: Tasks 3 and 12; no invalid events/sec gauge or timestamp-as-commit-time assertion.
- Early infrastructure feasibility, isolated resources, safe cleanup and non-skipping CI: Tasks 2, 7, 8, 9, 10, 12.
- Every approved removal category, external release/data preservation, lean public documentation and dependency cleanup: Task 9 plus final review in Task 13.
- One demo, recovery demo, language reuse: Task 10; no generator/SDK package dependency.
- Equivalent DB comparison, whole-pipeline measurement, warm-up/repetitions/environment/raw correctness evidence: Tasks 11–12.
- Article, README, truthful résumé/project copy, source links and publication drafts: Task 13; actual publication remains outside scope.
- All five Review Focus conditions have named owning tests. Interfaces shared between tasks are named above; old types are removed only after their replacement runtime is verified.

No important spec requirement is intentionally deferred inside the agreed implementation scope. A custom visual demo, supported SDKs, additional destinations, legacy data import, hosted deployment and actual publication remain explicit non-goals or later work.
