# IngestKit

A self-hosted event collector for trusted application backends. Send JSON over
HTTPS, buffer it in Redpanda, and store it in PostgreSQL with stable event IDs.
The repository includes copyable clients, a transactional outbox, failure tests
and reproducible experiments.

**HTTP 202 means broker acknowledgement. PostgreSQL storage is asynchronous.**
Retry uncertain delivery with the same UUID and content; storage keeps one row
per `(tenant_id,event_id)` and retains the first stored content.

<a href="docs/assets/deployment.svg"><picture><source media="(max-width:600px)" srcset="docs/assets/deployment-mobile.svg"><img src="docs/assets/deployment.svg" alt="A backend sends HTTPS to Caddy; API, broker, consumer and PostgreSQL run privately on one Compose host."></picture></a>

[Deployment](deploy/README.md) · [Application integration](docs/integration.md) ·
[Operations](docs/operations.md) · [Engineering article](docs/article.md)

## Deploy

Use a Linux host with Git, Docker/Compose 2.24.4+, OpenSSL and curl. Point your
own hostname at it and make ports 80/443 reachable. Replace `events.example.com`
below with that hostname. Go is built inside Docker.

~~~sh
git clone https://github.com/feat7/ingestkit.git
cd ingestkit
deploy/configure.sh events.example.com
docker compose --env-file deploy/.env -f deploy/compose.yml up -d --build --wait
~~~

Configuration is generated into ignored `deploy/.env` with mode 0600 and fresh
secrets; existing files are never overwritten. Only Caddy publishes ports.
See the [deployment guide](deploy/README.md) for DNS, resources and certificate
prerequisites. Public certificate issuance requires working DNS/reachability;
the automated HTTPS tests use a trusted private CA.

## Send and verify

For the initial generated key mapping, load the key without printing it and send
the three checkout events:

~~~sh
INGESTKIT_API_KEY=$(sed -n 's/^API_KEYS={"\([0-9a-f]*\)":"default"}/\1/p' deploy/.env)
curl --fail-with-body https://events.example.com/v1/events/batch \
  -H "Authorization: Bearer $INGESTKIT_API_KEY" \
  -H 'Content-Type: application/json' \
  --data-binary @examples/checkout/events.json
unset INGESTKIT_API_KEY

docker compose --env-file deploy/.env -f deploy/compose.yml exec -T postgres \
  psql -U ingestkit_admin -d ingestkit -c \
  "SELECT tenant_id,event_id,event_type FROM ingestkit.events ORDER BY persisted_at DESC LIMIT 10;"
~~~

Allow the consumer time to catch up before checking SQL. Repeating these fixed
UUIDs still stores three logical events for the initial tenant. Use fresh UUIDs
for new business events. Stop with the following command; named data volumes remain:

~~~sh
docker compose --env-file deploy/.env -f deploy/compose.yml down
~~~

## Integrate

Copy the standard-library [Python or Node client](docs/integration.md) into a
backend. For durable application delivery, run the [order/outbox example](examples/outbox/README.md):
business state and the serialized event are saved in one SQLite transaction,
then a separate dispatcher sends pending work with stable IDs. Keys stay in
trusted backend configuration. See the [API reference](docs/api.md) for limits,
responses and permanent versus uncertain failures.

## Inspect the evidence

The [corrected HTTPS study](benchmarks/results/selfhost-2026-11-04/README.md)
records 25 configurations with three measured repetitions: database batching,
payloads, duplicates, HTTP load, concurrency and three recovery scenarios.
All acknowledged IDs reconciled with stored content; generator limits and
ambiguous outcomes remain explicit. These local runs do not establish maximum
capacity. Batch INSERT had the lower median than COPY/staging/merge in all 11 DB
configurations, so the deployed writer now uses atomic batch INSERT.

To try an isolated local walkthrough, use Docker and Go 1.24+:

~~~sh
make demo DEMO_FLAGS=-cleanup
make demo-recovery DEMO_FLAGS=-cleanup
~~~

[Development and tests](docs/development.md) · [Experiment controls](benchmarks/README.md) ·
[Delivery semantics](docs/delivery.md) · [Architecture](docs/architecture.md)

This is a single-host deployment with one broker replica and finite log retention.
It does not protect against losing the host/disk or a recovery gap beyond retained
data. PostgreSQL rows have no automatic expiry; the operator owns backups, disk
monitoring and retention. It provides ingestion and replay-safe storage, without
an analytics UI, schema compiler or database row-level tenant policies.
