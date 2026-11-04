# Operate the collector

Start here when an application got HTTP 202 but you cannot yet find the event
in PostgreSQL. The API confirms that Redpanda accepted the event. Check the
worker's progress to find out whether storage is catching up or stuck.

## Check services, worker health and queued work

Run from the repository root. The `dc` helper selects this deployment and its
private configuration for every command:

~~~sh
dc() { docker compose --env-file deploy/.env -f deploy/compose.yml "$@"; }
dc ps
dc logs --no-color --tail 100 api consumer
dc exec -T redpanda rpk group describe ingestkit-consumer -X brokers=redpanda:9092
dc exec -T consumer /consumer check
~~~

API `/ready` checks whether the broker can accept writes. The consumer check
also needs database access and successful processing. A database Ping proves
connectivity, but cannot clear a failed write, quarantine operation or queue
position update.

The group command reports lag: queue records the consumer group has yet to
handle. It is not a unique count of business events. Compare it with the stored
row counts and latest insertion times below. The disk commands check space for
the database and queue:

~~~sh
dc exec -T postgres psql -U ingestkit_admin -d ingestkit -c \
  'SELECT tenant_id,count(*),max(persisted_at) FROM ingestkit.events GROUP BY tenant_id;'
dc exec -T postgres df -h /var/lib/postgresql
dc exec -T redpanda df -h /var/lib/redpanda/data
~~~

Application logs name the operation and safe error class, add SQLSTATE when
available, and identify the source topic, partition and offset. SQLSTATE is the
database's error code; the source coordinates locate the queue record. Startup
messages use partition/offset -1 when there is no record yet.

Repeated failures for an operation are throttled for ten seconds. The next
message reports how many were suppressed, and recovery is logged once. These
messages exclude raw errors, event payloads, credentials and database URLs.
Database and broker logs can contain data, so inspect them privately.

## Allow disk space for both the queue and the table

The event and quarantine topics keep records for seven days by default,
controlled by `RETENTION_MS=604800000`. Kafka removes old records asynchronously;
retention is not a disk quota. Monitor free space in both volumes, consumer lag,
traffic bursts and the backlog during an outage. An outage longer than the
retained log can leave events unrecoverable.

Editing the environment does not update an existing topic. To keep fourteen
days of records, apply the setting to both topics:

~~~sh
dc exec -T redpanda rpk topic alter-config ingestkit.events ingestkit.dlq \
  --set retention.ms=1209600000 -X brokers=redpanda:9092
~~~

Also set `RETENTION_MS=1209600000` in private configuration for future topic
creation. PostgreSQL rows do not expire automatically. Choose and test a database
archive/deletion policy before running continuously. The single host provides
no recovery from losing both its broker and database disks.

## Replace an API key without changing the tenant

Generate a key with `openssl rand -hex 32` in a private terminal. In
`deploy/.env`, temporarily map both the old and new key to the same tenant in
`API_KEYS`. Keep the JSON on one line and the file mode at 0600. Recreate the API:

~~~sh
dc up -d --no-deps --force-recreate --wait api
~~~

Move the backend callers to the new key and check that requests succeed. Then
remove the old key and run the same command again. The old key should return
401; existing events stay under the same tenant. A restart keeps the old
environment, so configuration changes need recreation. Changing the tenant
mapping changes identity instead of rotating a key. Keep keys out of browser
and public mobile code.

PostgreSQL's admin password is set when it first creates the data directory.
Changing that initial environment variable does not change an existing login.
To rotate database credentials, coordinate the SQL password change with private
configuration. Deliberately recreating the writer-role setup can change that
password; recreate the consumer afterward. Changing it during ingestion can
cause a temporary backlog.

## Back up the saved rows

A database dump contains rows committed at its snapshot. Events acknowledged
with 202 may still be in Redpanda and absent from the dump. Protect backups from
public access and keep a separate private copy of the deployment configuration.

~~~sh
umask 077
mkdir -p backups
dc exec -T postgres pg_dump -U ingestkit_admin -d ingestkit -Fc > backups/ingestkit.dump
~~~

## Restore into a new empty deployment

Keep the original deployment until the replacement is verified. Use a separate
checkout, configuration and Compose project name. Give it another hostname, or
stop the old gateway during cutover. Copy the dump to the replacement's private
`backups/` directory, then define its helper and run:

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

The replacement database must never have been migrated or populated. The restore
must fail on conflicts. The dump contains the migration ledger and table access
rules, so the writer role must exist before `pg_restore` restores those rules.
The commands create it with the replacement admin login; the `roles` service then
sets its private password and required privileges.

Keep `--clean`, schema drops and volume deletion away from the live source.
The integration test restores into a separate empty project and checks IDs,
payload content and new/duplicate ingestion while preserving the source.

## Fill the gap between an older backup and the queue

Restoring PostgreSQL does not rewind the worker's saved queue position. If the
broker is retained, its consumer group may already be past events missing from
the older snapshot. Stop the consumer and plan replay from the earliest retained
records with a fresh consumer group.

Use `dc stop consumer`, set a new `CONSUMER_GROUP=ingestkit-replay-UNIQUE` in private
configuration, then run `dc up -d --no-deps --force-recreate --wait consumer`.
A new group starts at the earliest retained offset. Keep that replacement group
name after checking the rows. Repeated event IDs are ignored against restored
rows. A separately reviewed offset reset while the worker is stopped is another
option; never reset an active group.

Retention expiry or broker disk loss can leave a gap that replay cannot fill.
A new empty broker contains none of the old host's pending events.

## Upgrade or shut down

Read the migration/dependency changes, make a backup and test the target revision
against a disposable restore. Pull that revision and run `dc up -d --build --wait`.
Keep compatible images and the previous revision for investigation. A code revert
does not reverse a database migration or broker format change; review those before
planning a rollback.

Use `dc down` to stop. Named database, broker and certificate volumes remain.
Routine shutdown does not need `down -v`, global prune or volume removal.

## What the recovery tests establish

The [recorded recovery experiments](../benchmarks/results/selfhost-2026-11-04/README.md)
run a database outage, worker kill/restart and broker-outage/outbox retry three
times each. They check accepted IDs and content and retain actual control times.
A completed start command does not prove the consumer group has caught up:
check readiness, lag and stored UUIDs. SQL observation gaps remain gaps. The
experiments do not establish recovery from host/disk loss or an outage beyond
retained history.
