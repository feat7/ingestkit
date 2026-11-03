# Architecture

The API authenticates a key, supplies tenant identity and receipt time, validates
an event envelope, and waits for Redpanda acknowledgement. It does not contact
PostgreSQL. The consumer reads bounded records and writes each valid segment with one multi-row
INSERT and ON CONFLICT DO NOTHING. The statement commits atomically, so a failed
segment cannot leave a partially reported write. It requires no staging table or
temporary-table privilege.

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
- **Batched SQL:** amortizes commits while preserving atomic failure and replay-safe
  conflict handling. COPY/staging/merge remains an explicit benchmark reference.

## Boundaries

This project does not provide analytics dashboards, a schema registry, generated
SDKs, automatic legacy migrations, database row-level tenant isolation, a hosted
service, or a connector platform. Small workloads may be better served by direct
PostgreSQL writes; the benchmark experiment compares the relevant write methods.

internal/persistence owns the new store. The API, messaging, and event packages
do not depend on generated code. internal/runenv is local test/demo
orchestration, not a runtime service dependency.

## What measurement changed

The [corrected self-hosted reference study](../benchmarks/results/selfhost-copy-2026-11-04/README.md)
found batch INSERT had the lower median elapsed time than COPY/staging/merge in
all eleven database configurations: batch sizes1–1000, payloads256B–16KiB and
0–90% duplicate submissions. The deployed writer now uses batched SQL; pipeline
measurements are repeated from the resulting committed code. This local result
does not establish a universal COPY crossover or compare JSONB with typed tables.
