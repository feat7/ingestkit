# How the collector works

An application sends an event to the HTTP API. The API puts it in Redpanda,
a Kafka-compatible event queue. A separate Go worker, called the consumer in
the code, reads the queue and saves rows in PostgreSQL. The queue lets the API
accept events while the database is temporarily unavailable. The API key
provides the tenant label stored with each event.

<a href="assets/event-journey.svg"><picture><source media="(max-width:600px)" srcset="assets/event-journey-mobile.svg"><img src="assets/event-journey.svg" alt="An application sends events over HTTPS. Redpanda keeps them for a worker that writes PostgreSQL rows."></picture></a>

This view follows the event. The [deployment map](assets/deployment.svg) shows
all five long-running services: Caddy, the Go API, Redpanda, the Go consumer and
PostgreSQL. The [compact map](assets/deployment-mobile.svg) and
[figure sources](figures/README.md) are also available. Startup jobs create the
topics, apply migrations and configure the writer login. Only Caddy publishes
host ports 80/443.

## The API waits for the queue

The API checks the backend key, uses it to select a tenant, validates the event
and adds receipt time. It returns 202 only after Redpanda acknowledges every
record named in that response. It never contacts PostgreSQL.

Publication has bounded buffering and a five-second deadline. The franz-go
producer uses idempotent production and waits for all in-sync replicas to
acknowledge. The supplied deployment has one replica.

## The worker saves rows, then its read position

The worker reads at most 500 records and handles each partition in order. It
saves each valid segment using one multi-row INSERT with
`ON CONFLICT DO NOTHING`. The statement commits all its rows together.

After that succeeds, the worker saves its position in the queue, called an offset.
A replacement worker resumes from the saved position. If the process stops after
SQL commits but before the offset commits, it reads the events again. The unique
`(tenant_id,event_id)` key keeps one row, preserving the first stored content.
An HTTP batch can cross partitions and SQL segments; the entire request is not
one database transaction.

A malformed broker record goes to a separate quarantine topic for inspection.
The worker waits for that write to be acknowledged before advancing past it.
A database failure causes a retry. [Delivery details](delivery.md) cover both paths.

## Find the code for each job

| Code | Job |
| --- | --- |
| `internal/api` and middleware | Check keys, apply per-tenant request limits, validate events and return HTTP responses |
| `internal/messaging` | Publish within buffer limits, read partitions, save bad records and advance offsets in order |
| `internal/persistence` | Write atomic SQL batches and ignore an already stored event ID |
| `internal/service` | Restart failed work, allow shutdown work to finish, report processing readiness and safe diagnostics |
| `deploy/` | Run the HTTPS services, generate settings, apply migrations and set up database roles |
| `internal/runenv` | Create and control isolated local test/demo/study projects; it is not a deployed service |
| `examples/` | Reusable HTTP callers and an application-owned SQLite outbox |
| `internal/benchmark` | Compare database writes, send HTTPS workloads and measure recovery |

PostgreSQL, Redpanda and Caddy use named volumes. The consumer login has CONNECT,
schema USAGE, SELECT and INSERT. It is not a superuser and needs neither TEMP nor
permission to create permanent database objects. Migrations have a separate admin
login. The private Compose network separates these services from public traffic.
HTTP tenant checks do not add PostgreSQL row-level isolation.

## Why the database writer uses INSERT

The worker uses one multirow INSERT per atomic batch with
`ON CONFLICT (tenant_id,event_id) DO NOTHING`. Replaying an event keeps its
existing row. This path needs no temporary-table privilege. Tests cover rollback,
duplicate IDs, restricted privileges and the SQL/offset crash gap.

The reference COPY benchmark creates a staging table, copies rows into it, then
merges them into the destination. Its total measures that complete transaction,
not plain COPY. The current [connection study](../benchmarks/README.md)
measures the deployed writer without claiming a universal speed advantage.

## Decide whether the queue helps your application

Redpanda holds events during PostgreSQL outages and retains records for replay.
It also adds a stateful service to operate. If direct PostgreSQL transactions
already solve the problem, that service may add little value. You can use the
outbox pattern independently of this collector.

The package runs on one Compose host with one broker replica and finite retention.
A database backup does not recover queued events lost with the only broker disk.
The package has no automatic high availability or recovery from host loss.
Applications own event IDs and business validation. Operators own disk growth,
backups and database archival/deletion. The saved events are available through SQL.
