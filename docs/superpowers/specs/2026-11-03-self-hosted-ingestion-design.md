# Self-hosted IngestKit: deployment, integration and experimental evidence

Status: proposal for approval. Native execution remains the user's chosen method.
This extends the completed showcase rather than replacing the collector again.

## Intent and user decisions

A reader should be able to deploy IngestKit, send application events to it, verify
storage and understand how to operate it when dependencies fail. The repository
and article should demonstrate those capabilities clearly enough for professional
review and technical readers. Reuse and deployability are now primary outcomes.

The user chose a self-contained Docker Compose server containing PostgreSQL and
Redpanda. They asked for better documentation and images, substantially more
experiments, and examples showing actual application integration.

Builder proposals, not additional user decisions: start with a single Linux host,
Caddy for public HTTPS, copyable Python/Node HTTP helpers, and one Python/SQLite
transactional-outbox example. Run experiments locally on owned resources before
considering an external server. Keep the Go ingestion runtime and HTTP contract.

## Approaches considered

1. **Self-contained Compose deployment — chosen.** One documented deployment
   contains the gateway, API, consumer, broker and database. Lowest setup burden;
   the operator still owns disks, secrets, upgrades, backups and a single-host
   failure domain.
2. **Application containers with existing database/broker.** Fits organizations
   that already operate PostgreSQL and Kafka-compatible infrastructure. More
   credentials, networking, version and retention choices. Keep runtime connection
   configuration compatible, but a supported managed-services matrix is later work.

A hosted SaaS, Kubernetes distribution, generated SDKs and a dashboard would add
independent products and are outside this iteration.

## Current findings and immediate correction

The current root Compose is a development environment with loopback ports,
local credentials and Redpanda's dev-container mode. It is not the deployment
package this user now wants. The examples are fixture-sending scripts, not an
application integration. Benchmark documentation still contains a stale sentence
saying evidence will be added; plotting is hardcoded to one historical directory.

A concrete durability error was found during this audit. The pinned Redpanda
v26.2.6 image's real process arguments include --unsafe-bypass-fsync=true when
started with dev-container mode. Setting topic write.caching=false did not remove
this process-level bypass. A network-isolated disposable probe confirmed that
--unsafe-bypass-fsync=false produces an actual false broker argument.

Correct this already-approved durability contract first, with a failing real
runtime regression. Record the effective bypass state in future manifests.
Preserve historical raw captures, clearly label their pipeline measurements as
development-mode results, and withdraw any implication that they establish
fsync-enabled broker throughput. PostgreSQL-only comparison samples retain their
recorded database settings; they are not replacement pipeline evidence. New
headlines require new measurements of the corrected code and configuration.

## Goals and completion boundaries

- A versioned Compose deployment starts from a fresh checkout using Docker,
  Compose, Git and the documented local secret-generation utility; Go is built
  inside Docker and is not a prerequisite on the deployment host.
- Only the HTTPS gateway is public. Database, Kafka, admin and metrics interfaces
  are private. Tests verify published ports, not only YAML text.
- First-use commands create configuration, start services, send one event and
  verify its UUID in PostgreSQL. Missing/placeholder configuration fails clearly.
- The integration helpers can be imported into application code, and the outbox
  example demonstrates business-state persistence, ambiguous delivery and retry.
- Experiments answer named questions with controls, at least three measured
  repetitions, raw evidence and correct ID reconciliation.
- Figures explain deployment, acknowledgement/replay and the measured results at
  normal README/article width. Every graph is generated from its cited data.
- The article is rewritten from the new evidence and the root README leads with
  use, deployment and integration. No inflated capacity/cost/adoption claims.

Non-goals: automatic remote provisioning, paid infrastructure, public DNS/domain
changes, package/registry publication, HA, multi-region operation, browser-visible
secret API keys, business schemas, automatic legacy-data import or broad connector
support. A single host is an explicit first deployment target, not a resilience
claim. No existing databases, volumes, releases or Git history are destroyed.

## Deployment package and operations

Keep the root development stack distinct. Add a deployment directory with one
canonical Compose configuration, gateway configuration, non-secret environment
template and a small noninteractive configuration script. The script accepts the
operator's hostname, generates fresh secrets into a mode-0600 ignored file, and
refuses to overwrite existing configuration. It does not install host software,
change the firewall, contact a server or print secrets into routine logs.

Use verified pinned image versions. The deployed broker must not enable fsync
bypass; topic write caching is disabled and PostgreSQL fsync/synchronous_commit
remain enabled. Inspect actual broker arguments and settings in a preflight and
integration check. Avoid inheriting other development-only behavior implicitly.
Document and test the Linux/container resource prerequisites without privileged
host-tuning scripts or invented minimum-resource/capacity claims.

Caddy fronts the ingestion routes and an intentionally chosen health route.
Public access to metrics, database and Kafka/admin ports is not configured. A
public hostname and reachable 80/443 are operator prerequisites for public ACME.
Local/CI tests use an explicit private test CA, verify certificates with that CA,
and exercise the same routing; they do not disable TLS verification or claim to
have tested real public DNS/certificate issuance.

Separate database migration/admin credentials from the consumer's limited writer
role. Grant the actual store's required TEMP/schema/table privileges and no
superuser access to the consumer. Initialization and grants are idempotent;
restarts preserve data. Ordinary configuration refresh recreates affected
services with new environment, rather than relying on restart to reload it.

Provide operator recipes for status, safe logs, accepted-but-pending events,
credential overlap/rotation, retention/disk growth, application upgrade and
volume-preserving stop. Add bounded structured consumer error diagnostics with
operation, safe error classification and source coordinates; no API keys,
payloads or credentialled DSNs. Keep readiness distinct from connectivity.

Provide a PostgreSQL backup/restore recipe and verify restore in a separate
owned stack. Document that restoring an older database with newer committed
broker offsets requires deliberate replay from retained data, while deduplication
handles repeats. Retention expiry or loss of the only broker disk can prevent
full recovery. Do not automate destructive offset resets or unsafe database
rollback on an operator's running deployment.

## Application integration

Publish a concise API reference with exact request/response/error examples,
limits, tenant mapping, stable IDs and the distinction between broker acceptance
and PostgreSQL storage. Secrets stay in trusted server-side configuration.
Browser applications call their own backend; this iteration does not introduce
public browser credentials or a new browser collector.

Refactor the existing Python/Node examples into small importable standard-HTTP
helpers plus their existing command-line walkthroughs. Callers provide the URL,
key and stable event IDs. Support single/batch requests, bounded timeouts,
retryable versus permanent failures, matching acknowledgement IDs and byte-stable
ambiguous retries. Return useful sanitized outcomes instead of silently dropping
failures. Keep these copyable reference modules, not new registry SDK releases.

Add one runnable order/outbox example using Python's standard SQLite support.
Creating an order and an outbox row is one local database transaction. Persist
the event UUID and serialized body before attempting HTTP. The dispatcher marks
broker acknowledgement only after matching 202 IDs, retains retryable/ambiguous
work across process restarts, and keeps permanent rejection visible for repair.
A 202 must never be labelled database-persisted. Document the example's single-
dispatcher scope and map its SQL pattern onto a reader's own application database.

Exercise the full example against the owned collector: normal delivery, broker
unavailability, dispatcher restart with pending events, stable-ID replay, and
one stored event per ID. Show the final SQL query and application/outbox state.

## Experimental design

The study runner owns one isolated stack at a time and has an explicit case
registry, progress journal and bounded cleanup. Standard cases use the corrected
broker settings and, once available, the deployment profile/gateway rather than
silently presenting direct-local-HTTP measurements as HTTPS deployment results.
Capture the route, effective settings, image digests, source SHA, host/VM resources,
seed, payload profile and all case controls. Preserve failures and interrupted
cases. A resumable study must reject mismatched source/configuration and never
reuse another run's data. Runs remain sequential for timing comparisons.

Do not run a full Cartesian product. Begin with these 25 deliberate configurations
and a small CI smoke subset. Each full configuration has warm-up and at least
three measured repetitions. Builder may settle safe durations after an untimed
feasibility check, but cannot silently drop cases or change a control when
comparing their results.

### Database: 11 configurations

Use 5,000 submissions per method/repetition. All three methods retain the same
schema, indexes, durability, first-content conflict policy and natural transaction
boundaries, with rotated method order. Generation, reset and verification stay
outside the timer.

| Question | Variable | Other controls |
| --- | --- | --- |
| Where does batching/COPY pay off? | batch 1, 10, 50, 100, 500, 1000 (6 cases) | 256-byte payload, 0% duplicates |
| What changes with larger payloads? | 1 KiB and 16 KiB (2 additional cases) | batch100, 0% duplicates; 256B baseline above |
| What does duplicate-heavy work cost? | 10%, 50%, 90% duplicate submissions (3 additional cases) | batch100, 256B; 0% baseline above |

Distinguish submitted/s from inserted unique/s. Duplicate cases perform fewer new
writes, so their elapsed ratios do not isolate a pure deduplication-overhead
percentage. Compare repetitions and report their variation.

### Pipeline: 11 configurations

Baseline: 256-byte payload, 1,000 offered events/s, HTTP batch100, concurrency32,
a ten-second window, two-second warm-up and a bounded post-window drain.

| Question | Variable | Additional cases |
| --- | --- | ---: |
| HTTP batching | batch1, 10, 100 | 3 including baseline |
| Offered load and observed limits | 100, 5,000, 10,000 events/s | 3 |
| Payload pressure | 1 KiB, 16 KiB, using batch10 | 2 |
| Burst versus steady | baseline count offered in first quarter of window | 1 |
| Client concurrency | 8 and 64 workers with batch1 at1,000/s | 2 |

For the payload sweep, use HTTP batch10 for both larger payloads and compare
against the 256B/batch10 case already in the batching sweep. This keeps the
request under 1 MiB and holds batch size constant; do not raise the API limit.
This is a builder decision resolving the fresh design check.

Actual generated/attempted load, queue misses, request failures and client limits
remain visible. A missed target is an observed limit, not proof of collector
capacity. Report HTTP ACK latency separately from committed-row visibility,
including p50/p95/p99, sample populations, complete timing intervals and drain.

Before increasing load, replace repeated full-ID-array polling with a connection-
owned incremental observer, such as a temporary pending-ID relation. A target
row only becomes observed after a completed query sees it committed. Receipt or
insertion timestamps alone are unsafe cursors: tests must cover transactions
committing out of order. Capture query duration, late polls and generator/resource
limits. Final reconciliation is independent of the observation bookkeeping.

Use deterministic varied payload contents for the new size study and record the
profile; historical repeated-x padding compresses unusually well. Exact byte
sizes, IDs and duplicate copies remain reproducible. Preserve per-request/event
samples in bounded raw captures; summary CSV/JSON is directly readable, while
large raw samples may be deterministically gzip-compressed with checksums.

### Recovery: three separate experiments

1. PostgreSQL stopped for five seconds during a fifteen-second 500-events/s run:
   acknowledgement, backlog and recovery/drain timeline.
2. Consumer forcibly restarted during that workload: pending IDs, group recovery
   and final reconciliation. This does not pretend every arbitrary kill lands
   inside the database/offset commit window; retain the deterministic crash test.
3. Broker unavailable long enough to exceed publish timeout while the application
   outbox retains 100 events: failed/ambiguous attempts, dispatcher restart,
   broker restoration and stable-ID replay to one stored row per event.

Recovery timelines have recorded fault/restoration times. They are availability
experiments, not headline throughput measurements. Count ambiguous/late deliveries
explicitly and never turn an accepted-but-missing event into a successful speed
result. Expected rejection/backpressure does not erase correctness failures.

## Documentation and visual presentation

Root README: short use case, topology figure, self-host quick start, first event,
links to application integration, measured results and operations. Keep technical
contracts in focused guides. Remove stale future-tense evidence text and repeated
setup instructions; retain historical planning and raw captures off onboarding.

Provide readable SVGs with optional PNG fallbacks: deployment/trust boundaries,
acknowledgement/commit/replay sequence, database batch crossover, pipeline latency
versus actual load, and recovery backlog timeline. Diagrams are code-backed;
charts are generated from the new study index, not hardcoded historical paths.
Use one typography/color system, direct labels and short captions. At normal
README width the labels must remain readable; test light/dark and narrow layouts.
Visual quality requires inspection, not merely a successful SVG export. No fake
screenshots, fabricated chart values or decorative architecture components.

Restructure the article around deploying/integrating the collector, the measured
tradeoffs and failure evidence. Explain which runtime/storage choices the new
results support and which remain provisional. Do not retain COPY for a performance
story if broader evidence contradicts it; a default-storage change, if justified,
needs a separate concrete decision with equivalent failure/duplicate tests.

## Verification and delivery

Run regression tests for changed behavior, real HTTPS deployment smoke, port and
role checks, first event/duplicate/restart preservation, outbox recovery, backup
restore and a small study correctness subset in CI. Keep full timing studies out
of ordinary CI. Native execution uses small verified commits and one fresh whole-
branch review, preserving the user's earlier execution choice.

Complete the corrected-contract commit before the expanded measurements. Commit
new harness code before measuring, pin results to that clean revision and publish
only values independently recomputed from raw samples. Every documented command
and local link must work from the new checkout. No actual cloud deployment,
package publication or social/article posting is authorized by this proposal.


## Design check and research

A fresh builder check identified that 100 ×16 KiB exceeds the current 1 MiB
request limit before envelope overhead. The revised payload sweep uses batch10
and its already-present 256B/batch10 control. This resolves the question without
changing the HTTP contract or confusing successful ingestion with rejection.
The remaining implementation choices are explicitly delegated above.

Primary references used for the deployment/operator-guide design:

- [Docker Compose in production](https://docs.docker.com/compose/how-tos/production/)
- [Caddy automatic HTTPS](https://caddyserver.com/docs/automatic-https)
- [Plausible Community Edition self-hosting guide](https://github.com/plausible/community-edition)
- [Redpanda startup mode flags](https://docs.redpanda.com/streaming/current/reference/rpk/rpk-redpanda/rpk-redpanda-start/)

The fsync finding was also reproduced against the pinned v26.2.6 container,
rather than inferred from current documentation alone. Both temporary audit
containers had networking disabled, no host ports or volumes, and were removed.
The corrective runtime regression failed with true and passed with false; the
full corrected integration/recovery suite passed in114.569s. This immediate
contract correction is separate from approval of the expanded design.
