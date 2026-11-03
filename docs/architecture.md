# Architecture

The API authenticates a key, supplies tenant identity and receipt time, validates
an event envelope, and waits for Redpanda acknowledgement. It does not contact
PostgreSQL. The consumer reads bounded records and writes valid segments through
a temporary PostgreSQL staging table using COPY, then merges into
ingestkit.events with ON CONFLICT DO NOTHING inside one transaction.

Offsets advance after the database transaction commits. A crash between those
two operations causes replay; the unique (tenant_id,event_id) key makes the
storage effect idempotent. This is not an atomic transaction across systems.

Malformed broker envelopes go to a separate acknowledged broker quarantine topic.
Database outages remain retries. A failed quarantine cannot justify committing
past the affected source record.

## Why these choices

- **HTTP boundary:** callers can reuse the service from any language without a
  generated client or schema compiler.
- **Broker buffer:** accepts bursts and permits database outages within retention,
  at the cost of an additional service and operational failure modes.
- **Generic JSONB table:** one readable storage path; application payload
  validation belongs to the application. No per-event or per-tenant table factory.
- **Sequential bounded processing:** makes offset progress straightforward to
  inspect. A claimed worker pool adds no value without a measured need.
- **COPY with staging/merge:** retains bulk loading while supporting replay.
  Its overhead must be measured against simpler writes, not assumed away.

## Boundaries

This project does not provide analytics dashboards, a schema registry, generated
SDKs, automatic legacy migrations, database row-level tenant isolation, a hosted
service, or a connector platform. Small workloads may be better served by direct
PostgreSQL writes; the benchmark experiment compares the relevant write methods.

internal/persistence owns the new store. The API, messaging, and event packages
do not depend on generated code. internal/runenv is local test/demo
orchestration, not a runtime service dependency.

## What measurement changed

The [local comparison](../benchmarks/results/local-2026-11-03/README.md) found
batch INSERT faster than COPY/stage/merge at batch size 100. The current store
keeps the latter implementation as a visible experiment; it is not justified
as a small-batch performance optimization. Larger batches, different payloads
or a direct PostgreSQL-only path need their own measurements. Generic JSONB
storage avoids generated schemas, but its cost relative to typed columns was
not measured.
