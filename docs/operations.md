# Operating a single-host collector

Run these commands from the repository root. The helper consistently selects
this deployment and its private configuration:

~~~sh
dc() { docker compose --env-file deploy/.env -f deploy/compose.yml "$@"; }
dc ps
dc logs --no-color --tail 100 api consumer
dc exec -T redpanda rpk group describe ingestkit-consumer -X brokers=redpanda:9092
dc exec -T consumer /consumer check
~~~

API `/ready` checks broker acceptance. The consumer's private readiness check
also requires database access and successful processing. A database Ping alone
cannot clear a write, quarantine or offset-commit failure. Application logs emit
an operation, error class, SQLSTATE when available, and source topic/partition/
offset. Repeated failures of an operation are throttled for ten seconds; the
next message reports the suppressed count. Recovery is logged once. Startup
failures use partition/offset -1 because no source record exists yet. Raw error
text, payloads, credentials and database URLs are excluded from these messages.
Database and broker logs can contain data; inspect those privately when needed.

Group lag shows queued broker records, not a durable count of business events.
To inspect persisted events without retrieving payloads:

~~~sh
dc exec -T postgres psql -U ingestkit_admin -d ingestkit -c \
  'SELECT tenant_id,count(*),max(persisted_at) FROM ingestkit.events GROUP BY tenant_id;'
dc exec -T postgres df -h /var/lib/postgresql
dc exec -T redpanda df -h /var/lib/redpanda/data
~~~

## Disk growth and retention

The event log and quarantine topic retain records for seven days by default
(`RETENTION_MS=604800000`). Kafka retention is asynchronous, not a storage quota.
Keep enough free space for traffic bursts and an outage backlog; monitor both
volumes, consumer lag and free disk. A long outage can exceed retention. Changing
the environment alone does not change existing topics: explicitly apply the
new retention value after deciding the required recovery window. For example,
apply a fourteen-day window with:

~~~sh
dc exec -T redpanda rpk topic alter-config ingestkit.events ingestkit.dlq \
  --set retention.ms=1209600000 -X brokers=redpanda:9092
~~~

Set the same `RETENTION_MS=1209600000` in private configuration for future topic
creation. PostgreSQL rows have no automatic
expiry. Choose and test your own archival/deletion policy before continuous use.
The collector does not protect against losing the single broker/database host.

## Rotate an API key

Generate a fresh key with `openssl rand -hex 32` in a private terminal. Edit
`deploy/.env` so `API_KEYS` temporarily maps both old and new keys to the same
tenant. Keep it valid JSON on one line and retain mode 0600. Recreate the API:

~~~sh
dc up -d --no-deps --force-recreate --wait api
~~~

Move backend callers to the new key and verify acceptance. Remove the old key
from the mapping and run the same command again. The old key should return 401;
existing stored events remain under the tenant. A container restart does not
reload configuration. Changing a tenant mapping changes identity; it is not key
rotation. API keys authenticate trusted backend callers, so do not embed them
in a browser or public mobile application.

Admin database credentials are initialized when PostgreSQL first creates its
data directory. Editing the initial password variable does not rotate an
existing login. Plan a coordinated SQL password change and private configuration
update if rotating database credentials. The writer role setup can update its
password when deliberately recreated, followed by recreating the consumer; do
not rotate it halfway through ingestion without expecting a temporary backlog.

## Back up and restore into a fresh target

A backup captures stored events at its snapshot. HTTP 202 may refer to events
still in the broker, so it is not proof they are in the dump. Keep the private
configuration separately and protect both backups from public access.

~~~sh
umask 077
mkdir -p backups
dc exec -T postgres pg_dump -U ingestkit_admin -d ingestkit -Fc > backups/ingestkit.dump
~~~

Restore into a **new empty deployment**, retaining the original until the new
one is verified. Use a separate checkout/configuration and Compose project name,
with a different hostname or the original gateway stopped during cutover. On the
new host/check-out, define a helper for the replacement project:

~~~sh
newdc() { docker compose -p ingestkit-replacement --env-file deploy/.env -f deploy/compose.yml "$@"; }
newdc up -d --wait postgres
newdc exec -T postgres psql -U ingestkit_admin -d ingestkit -v ON_ERROR_STOP=1 \
  -c 'CREATE ROLE ingestkit_writer LOGIN;'
newdc cp backups/ingestkit.dump postgres:/tmp/ingestkit.dump
newdc exec -T postgres pg_restore -U ingestkit_admin -d ingestkit \
  --exit-on-error /tmp/ingestkit.dump
newdc run --rm roles
newdc up -d --build --wait
~~~

Copy the dump into the replacement's private `backups/` directory first. These
commands assume the replacement database has never been migrated or populated;
`pg_restore` must fail on conflicts. The dump carries the migration ledger and
table ACLs. The restore needs the writer role to exist before restoring its ACLs:
the commands create it using the replacement's admin session before
`pg_restore`; the `roles` service then sets its private
password and required privileges. Do not use `--clean`, drop schemas or delete
volumes on the live source. The integration test exercises a separately owned
empty replacement and verifies restored IDs, payload content, and subsequent
new/duplicate event ingestion while the source remains intact.

After restoring an older database while retaining an existing broker, its
consumer group may already have committed past records missing from that
snapshot. Stopping the consumer and planning a replay is essential: use a fresh
consumer group from the earliest retained offsets. Stop the consumer with
`dc stop consumer`, add a fresh `CONSUMER_GROUP=ingestkit-replay-UNIQUE` in the
private configuration, and recreate it with
`dc up -d --no-deps --force-recreate --wait consumer`. The reader starts a new
group at the earliest retained offset. Keep the replacement group name after
verification. An alternative is an explicitly reviewed offset reset while the
consumer is stopped. Replayed IDs deduplicate
against restored rows. Retention expiry or broker disk loss can make the gap
unrecoverable. Never reset an active group or assume database restore rewinds
broker offsets. A new empty broker has no queued events from the old host.

## Upgrade and shut down

Read the migration and dependency changes, make a backup, and test the target
revision against a disposable restore first. Pull the chosen revision, then run
`dc up -d --build --wait`. Keep compatible images and the previous revision for
investigation; reverting code does not reverse database migrations or broker
format changes. Review those before planning a rollback.

Use `dc down` for shutdown. Named database, broker and certificate volumes remain.
Do not use `down -v`, global prune, or volume removal for routine operations.

## What the recovery evidence covers

The [corrected availability captures](../benchmarks/results/selfhost-2026-11-04/README.md)
retain three repetitions each for a database outage, consumer kill/restart and
application-outbox replay while the broker is unavailable. They reconcile accepted
IDs/content and record actual control times. A start command returning is not a
claim that the group has caught up; check readiness, lag and the stored UUIDs.
SQL-unavailable intervals remain observation gaps. These local tests do not
establish tolerance of host/disk loss or an outage beyond retained data.
