# Development

For deployment, use [deploy/README.md](../deploy/README.md). The root Compose file
is the local development stack with loopback dependency ports and public demo
credentials. It is distinct from the private HTTPS server package.

The module requires Go 1.24+. Container builds and CI pin Go 1.26.10. Docker with
Compose 2.24.4+, OpenSSL and Python 3.9+ are required by the complete local test
harness; Node is required for its caller tests (CI selects Node 24). PostgreSQL 18.6,
Redpanda 26.2.6 and Caddy 2.11.7 are the recorded deployment versions.

~~~sh
make up
make logs
make test
go vet ./...
go build ./...
python3 -m unittest discover -s examples/checkout -p 'test_*.py'
node --test examples/checkout/*.test.mjs
python3 -m unittest discover -s examples/outbox -p 'test_*.py'
python3 -m unittest discover -s benchmarks -p 'test_*.py'
make test-integration
make down
~~~

`make down` preserves normal development volumes. Integration tests create their
own projects, ports and volumes and verify ownership before control/cleanup.
They require the script's explicit setup flag and real services; missing setup
fails rather than silently skipping. Evidence stays under ignored `tmp/runs/`.
Never substitute a global Docker prune or reset another project's database.

The full integration gate covers trusted HTTPS and private ports, restricted SQL
privileges, first-event/content checks, denied-write readiness and recovery,
credential overlap/recreate, source-preserving backup restore, reusable callers,
outbox outage/after-ACK crash, broker/SQL failures, quarantine, rebalance and drain.
It also runs the small study, interruption/resume and all three real recovery
experiments. The process budget is 20 minutes; full timing studies stay out of CI.

## Configuration and migrations

The root Compose defaults work without an environment file. [.env.example](../.env.example)
lists local overrides. Runtime processes outside Compose require explicit
`API_KEYS` or `DATABASE_URL`, plus `BROKERS`. Other settings include `HTTP_ADDR`,
`METRICS_ADDR`, `EVENT_TOPIC`, `DLQ_TOPIC`, `CONSUMER_GROUP` and `RATE_LIMIT_RPS`.
Deployment configuration is a private data file, never shell-sourced by the harness.

Only `migrations/showcase` is applied, using the separate
`public.ingestkit_schema_migrations` ledger and `search_path=public`. Existing
legacy tables/ledgers are preserved; they are not automatically imported.

`/health` is liveness. API `/ready` probes broker acceptance. Consumer readiness
also latches failed storage/quarantine/progress operations; a successful Ping or
unrelated operation cannot clear them. Fix the cause and let the matching
operation succeed. A process restart starts new health state, so check actual
progress as well. Shutdown stops new polls and gives current work a ten-second
drain budget; exceeding it exits with failure.

## Walkthroughs and measurements

~~~sh
make demo DEMO_FLAGS=-cleanup
make demo-recovery DEMO_FLAGS=-cleanup
make bench-study-smoke STUDY_FLAGS='-out tmp/studies/my-smoke -seed 42'
~~~

Demos without `-cleanup` stop their own services and retain volumes; each prints
its exact cleanup command. They use development HTTP, while the registered
study and deployment integration checks use the self-hosted HTTPS profile and
explicit CA trust. See [examples](../examples/README.md) and the
[experiment guide](../benchmarks/README.md).

Full measurements require clean committed source, run sequentially and destroy
only their owned stacks after collecting evidence. Do not edit sources or run
other test/figure loads while timing them. Registered-study resume rejects
changed source, controls or runtime/image identity. [Figure generation](figures/README.md)
is optional development tooling, separate from runtime dependencies.

## Troubleshooting

- Port collision: change the matching development override or use an owned demo.
- 401: inspect the key mapping; preserve unsent application events.
- 503: inspect broker readiness and bounded publication pressure; retry stable IDs.
- 202 but no SQL row: check consumer readiness, lag, quarantine and the same UUID.
- Migration failure: inspect the actual ledger/error; do not force a version or delete volumes.

For the deployed package, follow the [operator recipes](operations.md), including
safe logs, key refresh and fresh-target restore. Both broker process fsync bypass
and topic write caching matter; the [historical capture correction](../benchmarks/results/local-2026-11-03/README.md)
is retained rather than rewriting old evidence.
