# IngestKit engineering showcase design

Date: 2026-11-03
Status: Proposed; awaiting user approval before implementation planning.
Repository baseline: `c6952ca` on `main`.

Design check: an independent Go/backend reviewer found no important unanswered
questions preventing implementation planning. Baseline verification on
2026-11-03: `go test -race ./...` passed using local cached dependencies. The
separately tagged full-stack integration suite was not exercised by that command.

## Intent and audience

Turn IngestKit into a compact, runnable engineering project that a hiring
team can assess quickly and an interested backend engineer can examine deeply.
The article and repository should demonstrate the author's understanding of
ingestion, acknowledgement boundaries, batching, duplicate delivery, backpressure,
and recovery. A second goal is a collector that other applications can use over
HTTP.

The user chose this general direction over building a broad ingestion product.
They explicitly questioned the value of the existing schema compiler. The
architecture and individual cuts below are proposed engineering decisions, not
separately approved user requirements.

Working description: **A small event ingestion pipeline in Go, with reproducible
experiments showing its performance and behaviour under failure.**

## Goals and completion criteria

1. A new reader can understand the problem and pipeline from a short README,
   start a local demonstration with one documented command, and find a submitted
   event in storage using the same identifier.
2. Every published delivery claim names the acknowledgement boundary and failure
   assumptions. There are no unconditional zero-loss or exactly-once claims.
3. A test can stop PostgreSQL, accept broker-acknowledged events, restore the
   database, and reconcile their identifiers without unexplained omissions.
4. Replaying a successfully persisted event creates no second stored event.
5. Broker, database, malformed-record, and shutdown failure paths have meaningful
   tests. A passing unit suite is never presented as proof of system recovery.
6. Benchmark commands record raw measurements, configuration, source revision,
   software versions, and correctness checks. Results distinguish HTTP acceptance
   from broker acceptance and database persistence.
7. One coherent walkthrough and small Python/Node HTTP examples demonstrate reuse
   without generating code or installing an IngestKit-specific client package.
8. A publishable article explains the measured decisions, limitations, and
   alternatives. Drafting and publication are separate completion states.

## Non-goals and anti-goals

- No hosted SaaS, analytics product, general connector framework, plugin system,
  schema registry, code generator, or multiple storage destinations.
- No custom dashboard in the initial scope. A readable terminal demonstration,
  actual results, and a clear architecture figure are sufficient. A visual demo
  can be considered after those work.
- No Kubernetes, orchestration platform, distributed worker framework, automatic
  partition creation, or broad cloud deployment coverage.
- No performance target chosen before measuring the corrected implementation.
- No invented production history or implied operational experience. The article
  distinguishes this project's experiments from production observations.
- Do not preserve a feature merely because it already exists. Do not replace
  deleted complexity with a new framework of similar size.

## Approaches considered

| Approach | Benefit | Cost / decision |
| --- | --- | --- |
| Focused engineering showcase with a reusable HTTP collector | Directly supports hiring, an article, and technical discussion; leaves a useful service | Selected direction. Limit supported runtime behaviour and prove it well. |
| Supported ingestion toolkit with compiler, SDKs, distributions, and extension points | Potentially serves a broader user base | Deferred. Requires compatibility and maintenance work without evidence of users needing it. |
| Presentation-only refresh of the existing implementation | Smaller initial change | Rejected as insufficient: underlying correctness and reproducibility problems would remain. |

## Architecture

Keep Go, Fiber, franz-go, pgx, PostgreSQL, Redpanda, and Docker Compose unless a
verified obstacle warrants changing them. This is a focused refactor, not a
framework migration.

```text
Application / curl
  -> Go HTTP API: authenticate, validate envelope, enforce limits
  -> Redpanda: acknowledge accepted records
  -> Go consumer: bounded batches, persist, then commit offsets
  -> PostgreSQL: one event table, idempotent writes

Malformed broker record
  -> Redpanda dead-letter topic, acknowledged before advancing its offset
```

The collector has one supported runtime write path. Database write alternatives
belong in the benchmark harness, not in an expanding set of runtime modes.

### Event and HTTP contract

Proposed breaking API for the refactored project:

- `POST /v1/events` accepts one envelope.
- `POST /v1/events/batch` accepts `{ "events": [...] }`, with the same envelope
  format for each item.
- An envelope requires `event_id` (UUID supplied by the caller), `event_type`
  (bounded non-empty string), `occurred_at` (RFC 3339 timestamp), and `payload`
  (JSON object). The server assigns `received_at` once when accepting the request.
- The server obtains `tenant_id` from its API-key mapping. Clients cannot choose
  or override it in the body. Tenant tagging and key scoping are not advertised
  as database row-level isolation or a complete multi-tenant security product.
- Retries retain the same event ID. Reusing an ID for a different logical event
  is a caller error; the first stored event wins. Payload equality checking is
  outside the initial contract and must be stated explicitly.
- `202 Accepted` means every record covered by that response was acknowledged by
  the configured broker. It does not mean PostgreSQL has stored it.
- Remove `sync=false` / fire-and-forget acceptance and the `?sync=` modes. Use a
  bounded publish deadline. A timeout/failure returns a non-success response and
  tells callers to retry the same IDs: a failed response can be ambiguous about
  whether some records reached the broker.
- Validate the whole HTTP batch before producing any records. A subsequent
  broker failure can still partially publish a batch. HTTP batches are not
  atomic broker transactions; retrying the same IDs is safe at the storage layer.
- Malformed input returns `400`, failed authentication `401`, oversized bodies
  `413`, tenant rate limiting `429`, and unavailable/saturated publication `503`.
  Error responses include a request ID without echoing secrets or full payloads.
- Keep `/health` for liveness and add `/ready` for relevant dependency readiness.
  The API checks broker readiness; the consumer checks database/broker readiness.
  Neither service claims readiness from merely having constructed a client.
- There is no public event-query API in this scope. Demo verification uses
  controlled local database queries, not an unprotected administrative endpoint.

Initial proposed limits: 256 KiB per event, 1 MiB per HTTP request, 100 events per
HTTP batch, a five-second broker publish deadline, and 64 MiB producer buffering.
The implementation plan must name franz-go options that enforce both byte and
record limits. Numeric tuning remains an implementation decision, documented and
bounded; removing limits to improve a headline result is not acceptable.

### Storage and duplicate delivery

- Use a new `ingestkit.events` table with `tenant_id`, UUID `event_id`,
  `event_type`, `occurred_at`, `received_at`, `persisted_at`, and `payload JSONB`.
- Use a unique key on `(tenant_id, event_id)`. Keep indexes minimal and motivated
  by the demo and its verification queries. Avoid per-event/per-tenant tables.
- Write a batch using PostgreSQL COPY into transaction-local staging, then
  `INSERT ... ON CONFLICT DO NOTHING` into the destination in the same
  transaction. Offset advancement follows successful commit.
- The design provides at-least-once processing with idempotent storage, subject
  to the documented retention and durability assumptions. It does not promise
  exactly-once execution across HTTP, Kafka, and PostgreSQL.
- Keep PostgreSQL `fsync` and `synchronous_commit` enabled. Retain normal broker
  durability settings. A single-node local broker is not highly available and
  cannot survive loss of its only disk. Replication settings must be recorded in
  any benchmark or deployment description.
- Ordered SQL migrations are the sole database schema authority. Remove competing
  initialization/code-generation/migration paths and swallowed migration errors.
  Use a pinned migration runner and fail startup on migration failure.

### Consumer progress, failures, and recovery

- Start with sequential bounded batch processing; do not advertise a worker pool
  or a timer that the implementation does not actually use. Batch size, poll
  bounds, cancellation, and rebalance handling must have real effects.
- Commit only a contiguous successfully handled prefix in each partition. Never
  commit a later offset past an unhandled record, including a malformed record.
- Database unavailability and unexpected storage failures keep the affected
  records uncommitted and retry with bounded, cancellable backoff. Exhausting a
  retry burst does not justify discarding data or sending an entire outage to a
  database-backed dead-letter table. The consumer's readiness becomes false.
- Deterministically invalid broker envelopes go to a dedicated broker dead-letter
  topic. Include original bytes/key and source topic/partition/offset plus a
  bounded error description. Advance the source offset only after that write is
  acknowledged. If the dead-letter write fails, keep the source record pending.
- A crash after database commit and before offset commit may replay records.
  Their stable IDs and unique key prevent additional stored rows.
- A configurable broker retention period bounds recovery. Surface increasing lag
  and document that a sufficiently long outage can exceed retention. Local demo
  recovery assertions always name the tested outage window.
- On shutdown, stop accepting requests, drain/finish work within a deadline,
  flush the producer, and close clients. If work cannot complete, leave it
  uncommitted and report shutdown failure; do not claim a successful flush.
- Provide documented commands to inspect quarantined records and replay repaired
  envelopes with stable IDs. A new replay platform or UI is unnecessary.

### Observability

Expose counters and histograms for HTTP requests/errors, broker publish outcomes
and latency, stored/duplicate/quarantined events, batch sizes, storage latency,
and retries. Use low-cardinality labels; never event IDs, arbitrary event types,
or tenant IDs as unbounded metric labels. Throughput comes from deltas of
counters over an explicit interval, not total events divided by time since the
last write. Report broker lag through the supported broker tooling or verified
client instrumentation. Do not make a full Grafana stack a startup prerequisite.

Use structured logs with request/event IDs where relevant and omit credentials
and full application payloads by default.

## Keep, replace, and remove

Cuts happen after replacement consumers/build paths exist, so intermediate
commits stay testable. Preserve history in Git instead of adding an archive tree.

| Current material | Planned treatment |
| --- | --- |
| `cmd/api`, `cmd/consumer`, `internal/messaging`, useful middleware and logging | Refactor around the small contract; retain useful tested behaviour. |
| `internal/storage` and current generated storage | Replace with generic transactional event storage and broker quarantine behaviour. |
| `cmd/cli`, `internal/schema`, `schema/`, `generated/` | Remove after API/consumer/Docker/CI no longer depend on them. No compiler or generated SDKs in the new workflow. |
| `internal/validation` | Replace schema-driven validation with envelope and request-limit validation. Remove its compiler dependency. |
| `internal/config` | Replace CLI project detection with only configuration actually consumed by running services, if a shared package is useful. |
| `packages/npm`, `packages/python` | Remove installer wrappers and placeholder exports from the repository. Do not delete or overwrite existing external package releases. |
| `examples/blog-flask`, `examples/ecommerce-express`, old quickstarts and delivery guides | Replace with one small scenario and tiny standard-HTTP Python/Node examples. |
| `PROJECT_OVERVIEW.md`, `CLAUDE.md`, `docs/README.md`, generator and Atlas guides | Remove stale/duplicate material; link to the consolidated documentation below. |
| `docs/development.md` | Rewrite around the actual supported commands. |
| `docs/docker-deployment.md` | Fold local startup guidance into README/development; do not retain unsupported production promises. |
| `docs/load-testing.md`, `loadtest/`, `loadtest-scenarios.yaml`, `tests/benchmarks` | Replace with one coherent benchmark suite, real raw output, and a short methodology. Keep a tool only if it serves that suite. |
| `atlas.hcl`, `Dockerfile.migrate`, `scripts/migrate.sh`, old generated migrations, `init-db/` | Consolidate into one ordered migration path. Existing data policy below controls handling of old migrations/tables. |
| `scripts/tune-system.sh` | Remove host-wide privileged tuning as a project prerequisite. |
| Makefile, Compose, Dockerfiles, `.dockerignore`, `.env.example` | Simplify together; one default startup path, explicit limits, no unused configuration, no hidden errors or destructive resets. |
| CI and release workflows | Replace compiler/package publishing steps with actual build, unit, integration, and demo checks. No package registry publishing in initial scope. |
| README, CONTRIBUTING, LICENSE, CODE_OF_CONDUCT | Rewrite README and shorten contributing instructions; preserve license and conduct policy. |

### Compatibility and data policy

This is an explicit breaking redesign of an experimental project. There are no
known supported external users in the current discussion; compatibility is not
assumed or silently promised. Explain removed commands and changed event format
in the eventual release notes/README transition note.

Do not drop existing tables or volumes, reset a local database, rewrite Git
history, delete remote releases, or unpublish packages. The new schema lives in
`ingestkit.events`, leaving old `events_*` tables untouched. Before replacing
migration history, inspect the configured migration ledger and use a new
explicitly named ledger for the new schema so existing version numbers cannot
silently collide. If legacy tables/data are discovered during execution,
preserve them and document the opt-in transition; automatic legacy data import
is outside this scope.

Run demonstrations, tests, and destructive failure experiments in uniquely
named Compose projects with their own volumes/ports. Cleanup targets address
only resources created for that run. Normal stop commands preserve volumes.

## Demonstration and reuse

Use one checkout/product-events scenario: `product_viewed`, `checkout_started`,
and `order_completed`. These are payload examples, not separate application
servers, generated classes, or database tables.

`make demo` should start its isolated local stack, wait on readiness, send a
small deterministic set of events, print accepted IDs, query stored IDs, and
show a small SQL funnel result. It finishes with an explicit reconciliation and
preserves or clearly reports the location of its evidence. Failure produces a
nonzero exit code. Normal demo execution need not inject outages.

`make demo-recovery` should run a separate isolated scenario that pauses database
availability, sends broker-acknowledged IDs, shows lag, restores availability,
and verifies all accepted IDs arrive once in storage within a bounded deadline.

Small Python and Node scripts use standard HTTP facilities and exactly the same
API contract. They demonstrate reuse across application languages without
claiming adoption or pretending to be maintained SDK distributions.

## Benchmark design

Two explicitly separate experiments prevent misleading comparisons:

1. **Database write methods:** compare single-row INSERT, batched INSERT, and
   COPY-to-staging plus idempotent merge using the same table shape, payloads,
   indexes, duplicate policy, durability configuration, and expected IDs. Exclude
   dataset generation and cleanup from timed sections. Record successful and
   duplicate row counts; correctness failure invalidates the run.
2. **Whole pipeline:** measure broker-acknowledged HTTP acceptance, persisted
   unique events/sec, acknowledgement latency, receipt-to-persistence latency,
   backlog/drain time, failures, and resource use under steady and burst load.
   Measure recovery separately. Do not call API response time end-to-end latency.

Use seeded workloads, explicit payload sizes and batch sizes, warm-up, at least
three measured repetitions, per-run raw output, and a fixed source revision.
Record medians and p95/p99 for clearly defined latency samples. The local
latency experiment runs on one host/clock; multi-host clock assumptions must be
named if introduced later. State whether the load generator shares the host and
whether the test is CPU, disk, or client limited. Use a dedicated database/run
namespace; never truncate unrelated tables.

Store bounded raw JSON/CSV and environment metadata under
`benchmarks/results/<run-id>/`; large captures stay outside Git with a documented
location/checksum. No throughput/cost headline is restored before a matching
result is committed. Do not rent infrastructure or incur paid-service costs as
part of this scope without separate authorization.

## Documentation and article

The public reading path is deliberately small:

- `README.md`: problem, current status, architecture figure, start/demo command,
  one event example, verified result if available, limitations, and deeper links.
- `docs/architecture.md`: component responsibilities, choices, and alternatives.
- `docs/delivery.md`: acknowledgement, retry, duplicate, quarantine, retention,
  failure-window, and recovery behaviour.
- `docs/development.md`: prerequisites, exact commands, configuration, tests,
  migrations, and troubleshooting demonstrated by the project.
- `benchmarks/README.md`: methodology, commands, environment, and measured results.
- `docs/article.md`: publishable narrative linked to a fixed code revision and
  actual experiment artifacts.

Superpowers design/plan records stay under `docs/superpowers/` and do not become
part of the short onboarding path. Avoid moving duplicate prose between files.
Test/copy-check public commands against a fresh isolated setup.

Working article angle: **Building an event ingestion pipeline in Go—and testing
what survives a crash.** Structure it around the initial problem, simple
baseline, measured batching decision, broker boundary, failure experiments,
limitations, and how to reproduce. Describe the compiler removal as a design
choice only if it teaches something useful. Never invent a historical sequence
of optimizations from newly created comparisons.

Reference models already researched:

- Tiger Data: https://www.tigerdata.com/blog/benchmarking-postgresql-batch-ingest
- Segment: https://www.twilio.com/en-us/blog/insights/exactly-once-delivery
- PostHog: https://posthog.com/docs/how-posthog-works/ingestion-pipeline

The proposed publication path is a full article on the author's blog, an
optional DEV cross-post with a canonical URL, and a regular HN submission for the
article. A runnable project can separately be considered for Show HN. Publishing,
posting, account creation, or purchasing hosting are not authorized by this
implementation design and are deferred to a concrete later request.

## Phases and dependency gates

These are design milestones. After approval, a separate implementation plan
will translate them into exact files, test cases, commands, and small commits.

1. **Baseline and contract:** record current tests, preserve legacy data, define
   the new envelope/response contract and test fixtures. Identify replacement
   import/build paths before deletion. Exit: reviewed contract and baseline.
2. **Storage and ingestion core:** introduce new migrations, idempotent
   transactional storage, bounded broker-acknowledged HTTP ingestion, and the
   replacement consumer. Exit: real event reaches PostgreSQL with the same ID;
   duplicate processing is safe; existing useful checks still pass.
3. **Failure semantics:** implement/test retry, partition progress, broker
   quarantine, rebalance, cancellation, readiness, and shutdown. Exit: automated
   failure matrix passes against real isolated broker/database services.
4. **Remove and consolidate:** delete obsolete code/docs/distribution paths,
   simplify Docker/Make/config/CI, and remove unused dependencies. Exit: no
   stale imports/commands; new startup and checks work without generated files.
5. **Demo and reuse:** add deterministic walkthrough, recovery demo, and small
   Python/Node callers. Exit: a fresh reader can reproduce the same results;
   the three callers agree on the API.
6. **Measurements:** run the database and whole-pipeline experiments, save raw
   evidence and metadata, and investigate correctness failures before reporting
   speed. Exit: repeatable verified results; no unexplained missing events.
7. **Writing and release preparation:** finish concise docs, architecture figure,
   article, demo captures, and a factual project/resume description. Exit: all
   claims trace to code/tests/results; every public setup command was exercised.
   Prepare publication material without publishing it.

Do not wait until the end to learn that Compose or the full pipeline cannot run.
Establish that ability during phase 2. Each phase ends with appropriate fresh
verification and a small reviewable commit; do not request permission for every
ordinary step once the design and plan are approved.

## Verification matrix

| Scenario | Required observation |
| --- | --- |
| Valid single/batch request | Broker acknowledgement before 202; original IDs eventually stored. |
| Invalid/oversized/unauthenticated request | Defined error; invalid data is not produced. |
| Broker unavailable or producer buffer full | Bounded non-success response, no false acceptance; stable-ID retry reconciles ambiguous sends. |
| PostgreSQL unavailable | Accepted events remain pending in the broker; lag grows; retry cancellation works. |
| PostgreSQL restored | Pending accepted IDs persist without extra stored rows within the tested retention window. |
| Crash after DB commit before Kafka offset commit | Replay occurs safely; unique stored ID count stays correct. |
| Malformed record between valid records | Acknowledged quarantine or blocked source progress; later commits cannot skip the failed record. |
| Dead-letter broker write fails | No source offset advance; no false quarantine-success metric. |
| Rebalance / graceful shutdown / forced termination | No commit past unfinished work; recovery reconciles accepted IDs. |
| Same event ID retried / same ID in another tenant | One row per tenant and event ID; tenant comes only from credentials. |
| Benchmark and demo cleanup | Only the run's own resources/data are changed. |

Run unit tests with the race detector where supported, build all commands, and
run the real integration suite in CI with PostgreSQL and Redpanda. Integration
jobs explicitly fail rather than silently skip when services or fixtures are
missing. Keep expensive throughput measurements separate from normal CI; use
small correctness/recovery cases on every PR.

## Decisions left to the implementation plan / builder

The builder may choose internal package boundaries, exact SQL/index details,
bounded backoff constants, metric library, test failure-injection seams, and the
benchmark load tool. These choices must serve the contract above and avoid new
platform work. Dependency/image versions must be checked against primary
documentation at implementation time instead of guessed from old files.

Important unverified conditions: full-stack operation has not been tested in
this session, Docker daemon access requires sandbox permission, no valid new
performance number exists, and no external-user/data inventory has been
established. Preserve data and external artifacts by default. These limitations
are work to verify, not reasons to manufacture successful results.
