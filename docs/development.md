# Development

Requires Go 1.24+ and Docker with Compose v2. The pinned container build uses
Go 1.26.10; PostgreSQL is 18.6 and Redpanda is 26.2.6. Python and Node are optional
for their integration examples.

~~~sh
make up                 # build, initialize topics/migrations, wait for readiness
make logs
make test               # service-independent race tests
go vet ./...
make test-integration   # real broker/database, isolated ports and volumes
make down               # preserve normal local volumes
~~~

Integration tests fail when invoked without their explicit setup flag; they do
not silently skip. The script supplies the flag and the test runner owns its
stack. Cold startup waits for PostgreSQL TCP readiness; migration failure stops
the stack from becoming ready.

## Configuration

The local Compose defaults work without a copied .env file. .env.example
lists local overrides. Outside Compose, service processes require explicit
API_KEYS (JSON key-to-tenant mapping) or DATABASE_URL, plus BROKERS.
Relevant settings are HTTP_ADDR, METRICS_ADDR, EVENT_TOPIC, DLQ_TOPIC,
CONSUMER_GROUP, and RATE_LIMIT_RPS. Local defaults are not production
credentials. The API listens on 8080; consumer readiness/metrics on 8081.

/health is liveness. /ready probes dependencies. Metrics use counters and
latency histograms with bounded labels; compute throughput from counter deltas
over a stated interval. Logs can include request/event IDs at debug level,
without keys or full payloads.

## Data and migrations

Only migrations/showcase is applied, using the ledger
public.ingestkit_schema_migrations and explicit search_path=public.
The new table is ingestkit.events. Older events_* tables and migration
ledgers are not dropped or automatically imported.

The default Compose project is ingestkit-showcase, avoiding legacy volumes.
Tests reserve distinct loopback ports per run and verify ownership before
resource control/cleanup. Evidence is saved under tmp/runs/<project>/.
Do not run global Docker prune commands or reset other projects.

## Troubleshooting

- Port in use: change the matching port in .env, or use an isolated demo/test.
- 401: check the configured key mapping.
- 503: inspect /ready, service logs, broker availability and buffer pressure.
- Accepted but not stored: inspect consumer readiness, broker group lag and
  quarantine. An API acknowledgement alone is not a storage check.
- Migration error: inspect the actual error/ledger; do not force a version or
  discard a volume to conceal it.
