# IngestKit

A small event pipeline in Go for exploring acknowledgement, batching, duplicate
delivery, and recovery.

Send an event over HTTP, buffer it in Redpanda, and store it in PostgreSQL with
the same UUID. The interesting part is what happens when a dependency fails.

~~~mermaid
flowchart LR
  A[Application] -->|JSON + stable UUID| B[Go API]
  B -->|broker acknowledgement| C[Redpanda]
  C --> D[Go consumer]
  D -->|COPY + idempotent merge| E[(PostgreSQL)]
  D -->|invalid broker record| F[Quarantine topic]
~~~

## Run locally

Requires Docker with Compose v2. Go 1.24+ is needed for development and the
automated demonstrations.

~~~sh
git clone https://github.com/feat7/ingestkit.git
cd ingestkit
make up
~~~

For an isolated walkthrough that verifies stored IDs:

~~~sh
make demo
make demo-recovery
~~~

Or send an event to the manually started stack:

~~~sh
curl --fail-with-body http://localhost:8080/v1/events \
  -H 'Authorization: Bearer local-demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"00000000-0000-4000-8000-000000000001","event_type":"order_completed","occurred_at":"2026-11-03T00:00:00Z","payload":{"order_id":"order-1","amount":25}}'
~~~

**202 means the broker acknowledged the event.** Database persistence happens
afterward. Retry an uncertain request with the same event ID; storage keeps one
row per tenant and ID. An HTTP batch can partially reach the broker on failure.

~~~sh
docker compose exec postgres psql -U ingestkit -d ingestkit_test \
  -c "SELECT event_id, event_type, payload FROM ingestkit.events;"
make down
~~~

Stopping preserves volumes. The local stack uses a separate ingestkit-showcase
project, local-only credentials, and loopback ports.

## What is verified

Tests cover validation, bounded publishing, duplicate storage, database and broker
outages, a crash between database and offset commits, quarantine failures,
rebalance, and shutdown. Run make test and make test-integration.

This is an engineering reference project. The local broker has one replica;
disk loss and outages beyond broker retention are outside its recovery guarantee.
There is no unconditional zero-loss or exactly-once claim. New benchmark results
are not published until the reproducible measurement work is complete.

Read [architecture](docs/architecture.md), [delivery semantics](docs/delivery.md),
or [development instructions](docs/development.md).

The previous compiler, generated SDKs, registry installers and event-specific
endpoints have been retired. Existing package releases and legacy data are not
deleted or automatically migrated.

Apache 2.0 — [LICENSE](LICENSE).
