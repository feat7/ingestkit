# Develop and test IngestKit

For a server, follow [deployment](../deploy/README.md). The root Compose file is
for local development: it exposes dependency ports on loopback and uses public
demo credentials. The separate deployment package provides private services
behind HTTPS.

## Start the local stack and run checks

Use Go 1.24+, Docker with Compose 2.24.4+, OpenSSL and Python 3.9+. The complete
harness also needs Node for the caller tests; CI selects Node 24. Container builds
and CI pin Go 1.26.10. The recorded deployment runs PostgreSQL 18.6, Redpanda 26.2.6
and Caddy 2.11.7.

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

`make down` keeps the development volumes. Integration tests create their own
projects, ports and volumes, check ownership, then control or remove only those
resources. Use the integration script, which supplies the required setup flag;
tests fail if that setup is missing. They do not silently skip real-service
checks. Private evidence goes under ignored `tmp/runs/`. A global Docker prune
or reset of another project's database is never part of test cleanup.

The full integration suite checks trusted HTTPS and published ports, restricted
writer privileges, stored IDs/content, readiness after denied writes, key overlap
and recreation, backup restore into a separate target, reusable callers, outbox
outage/after-ACK crash, broker/SQL failures, quarantine, rebalance and shutdown.
It also runs the small study, interruption/resume and the three recovery scenarios.
The Go process has a 20-minute budget; full timing studies stay out of CI.

## Configure a local process

The root Compose defaults work without an environment file. [.env.example](../.env.example)
lists overrides. Outside Compose, the API needs `API_KEYS`, the consumer needs
`DATABASE_URL`, and both need `BROKERS`. Other settings are `HTTP_ADDR`,
`METRICS_ADDR`, `EVENT_TOPIC`, `DLQ_TOPIC`, `CONSUMER_GROUP` and `RATE_LIMIT_RPS`.
The deployment harness reads its private settings as data; it never sources that
file as a shell script.

Only `migrations/showcase` runs. Applied migrations are recorded separately in
`public.ingestkit_schema_migrations` with `search_path=public`. Existing legacy
tables and migration records stay untouched; there is no automatic legacy import.

## Check processing, not just connectivity

`/health` checks that a process is alive. API `/ready` checks broker acceptance.
The consumer also remembers failed storage, quarantine and queue-progress
operations. An unrelated success or a database Ping cannot clear those failures.
Fix the cause and let the matching operation succeed. A new process starts with
fresh health state, so inspect progress after a restart too.

Shutdown stops new polls and gives the current work ten seconds to finish. If
that budget expires, the process exits with failure.

## Run the walkthroughs and experiments

~~~sh
make demo DEMO_FLAGS=-cleanup
make demo-recovery DEMO_FLAGS=-cleanup
make bench-study-smoke STUDY_FLAGS='-out tmp/studies/my-smoke -seed 42'
~~~

Without `-cleanup`, demos stop their own services and preserve volumes, then
print the command to remove that run. Demos use development HTTP. The registered
study and deployment checks use the HTTPS profile and explicitly trust its local
certificate authority. See [examples](../examples/README.md) and the
[experiment guide](../benchmarks/README.md).

Before full measurements, commit all source and run studies sequentially. Each
run collects evidence and destroys only its own stack. Editing source or running
other tests/figure jobs during timing changes the comparison. Resume rejects
changed sources, controls and runtime/image identity. [Figure tools](figures/README.md)
are optional development dependencies; the collector does not need them.

## Troubleshoot a local run

- Port collision: change the matching development setting or use an isolated demo.
- 401: check the key mapping and keep unsent application events.
- 503: check broker readiness and publication pressure, then retry the same IDs.
- 202 with no SQL row: check worker readiness, queued work and quarantine; look for the same UUID.
- Migration failure: inspect the ledger/error before changing anything; do not force a version or delete volumes.

The [operator guide](operations.md) has the deployed procedures for logs, key
rotation and fresh-target restore. Both the broker process's fsync bypass and
topic write caching affect durability. Previous measurement bundles have been
archived outside the checkout and withdrawn from public claims. See the
[measurement guide](../benchmarks/README.md) for the revalidation work.
