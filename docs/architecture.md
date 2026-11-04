# Architecture

The deployment has five long-running services: Caddy, a Go API, Redpanda, a Go
consumer and PostgreSQL. Initialization jobs create topics, apply migrations and
set up the restricted writer login. Only Caddy publishes host ports 80/443.

<a href="assets/deployment.svg"><picture><source media="(max-width:600px)" srcset="assets/deployment-mobile.svg"><img src="assets/deployment.svg" alt="Public HTTPS terminates at Caddy; the collector and its dependencies stay private on one Docker Compose host."></picture></a>

[Full diagram](assets/deployment.svg) · [Source and reproduction](figures/README.md)

The API maps a backend key to a tenant, validates the envelope, supplies receipt
time and publishes to Redpanda. It returns 202 after all records covered by the
response are acknowledged. It never contacts PostgreSQL. The producer uses
bounded buffering, a five-second publish deadline, all-ISR acknowledgements and
franz-go's idempotent producer.

The consumer reads at most 500 records, handles each partition in order and
writes valid segments with one multi-row INSERT plus `ON CONFLICT DO NOTHING`.
Each SQL statement commits atomically. Only then does the consumer commit those
source offsets. A crash in between can replay the segment; the unique
`(tenant_id,event_id)` key prevents another row. An HTTP batch can span partitions
and SQL segments; it is not one transaction across the entire pipeline.

Malformed broker envelopes go to a separate quarantine topic on the same broker.
The consumer waits for its acknowledgement before advancing the source offset.
Database failures are retries, not quarantine. See [delivery semantics](delivery.md).

## What the components own

| Component | Responsibility |
| --- | --- |
| `internal/api` and middleware | Authentication, per-tenant request limiting, validation, HTTP acknowledgements |
| `internal/messaging` | Bounded publish, partition processing, acknowledged quarantine, offset ordering |
| `internal/persistence` | Atomic batched SQL and first-content conflict handling |
| `internal/service` | Supervision, drain, processing readiness and safe diagnostics |
| `deploy/` | Canonical HTTPS Compose package, configuration generation, migrations/role initialization |
| `internal/runenv` | Owned local test/demo/study orchestration; not a deployed service |
| `examples/` | Importable callers and an application-owned SQLite outbox |
| `internal/benchmark` | Explicit write comparisons, HTTPS load observation and recovery experiments |

PostgreSQL, Redpanda and Caddy state use named volumes. The consumer has CONNECT,
schema USAGE, SELECT and INSERT privileges; it is not a superuser and needs no
TEMP or permanent-DDL privilege. Migrations use a separate admin login.
The private Compose network is the deployment trust boundary; tenant API scoping
does not provide PostgreSQL row-level isolation.

## Why the writer changed

The first [complete corrected study](../benchmarks/results/selfhost-copy-2026-11-04/README.md)
compared single INSERT, batch INSERT and the actual temporary-table COPY/merge
path. Batch INSERT had the lower median than COPY/merge in all 11 configurations.
The default was changed separately, retaining atomic rollback, deduplication,
restricted-role and deterministic crash-replay checks. The COPY transaction now
lives as an explicit benchmark reference; changing the default does not relabel
a different algorithm as COPY.

A [new complete study](../benchmarks/results/selfhost-2026-11-04/README.md) records
the resulting committed implementation. Repetition ranges overlap or reverse
some individual timings. There is no universal/adaptive batch-size threshold or
measured JSONB-versus-typed-table advantage.

## Scope and tradeoffs

The broker permits asynchronous acceptance and retained-log recovery during
PostgreSQL outages, at the cost of another stateful service. For an application
already satisfied by direct transactional PostgreSQL writes, that extra service
may not be useful. The outbox pattern is reusable independently of this collector.

This package targets one Compose host. One broker replica, finite log retention
and a database backup do not provide high availability or automatic recovery from
losing the only disks. There is no analytics dashboard, connector catalog, schema
registry, generated SDK, automatic legacy import or business-payload validator.
Applications own their business schemas and event IDs; operators own backups,
disk growth and any database archival/deletion policy.
