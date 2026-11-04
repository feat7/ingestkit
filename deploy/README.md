# Run IngestKit on one server

This Compose deployment runs Caddy, the Go API, Redpanda, the database worker and
PostgreSQL on one Linux host. Caddy exposes HTTPS; the other services have no
public ports. Docker builds the Go programs for you.

## Prepare the host

Install Git, Docker Engine with Compose v2.24.4+, OpenSSL and curl. Point a
hostname you own at the server, and make ports 80 and 443 reachable. Replace
`events.example.com` in every command with that hostname. Caddy uses DNS and
those ports to obtain the public certificate, then keeps it in a named volume.
Local automated tests use a private certificate authority; they do not test
public certificate issuance.

Allow memory for Redpanda's 1 GiB allocation plus PostgreSQL, Go and Docker.
The [measured environment](../benchmarks/results/selfhost-2026-11-04/manifest-1.json)
had a Docker VM with 10 CPUs and about 16 GiB RAM; that is not a minimum sizing
recommendation. Disk use depends on traffic and retention. Both the queue and
database live on this host, so losing its disk can lose both.

## Create private settings and start

Run this once from a fresh checkout:

~~~sh
git clone https://github.com/feat7/ingestkit.git
cd ingestkit
deploy/configure.sh events.example.com
docker compose --env-file deploy/.env -f deploy/compose.yml config --quiet
docker compose --env-file deploy/.env -f deploy/compose.yml up -d --build --wait
~~~

The configuration script needs a POSIX shell, standard utilities and OpenSSL.
It creates deploy/.env with file mode 0600, three independent secrets and an API
key mapped to tenant default. It neither contacts the server nor installs software.
It refuses to overwrite an existing configuration, so keep that file private,
out of Git and backed up.

An optional second argument to the script chooses another configuration directory.
The hostname localhost is accepted for local TLS verification. API_KEYS is a
JSON object; repeated key names are rejected even if an escaped spelling decodes
to the same key. To rotate a key, use two distinct keys for the same tenant.

The database, broker, administration and metrics stay on Docker's private network.
The consumer login `ingestkit_writer` has schema usage, SELECT and INSERT, without
temporary-table privileges. Migrations run with a separate admin login.

## Send the checkout events and find their rows

For the initially generated key mapping, load the key without printing it:

~~~sh
INGESTKIT_API_KEY=$(sed -n 's/^API_KEYS={"\([0-9a-f]*\)":"default"}/\1/p' deploy/.env)
curl --fail-with-body https://events.example.com/v1/events/batch \
  -H "Authorization: Bearer $INGESTKIT_API_KEY" \
  -H 'Content-Type: application/json' \
  --data-binary @examples/checkout/events.json
unset INGESTKIT_API_KEY

docker compose --env-file deploy/.env -f deploy/compose.yml exec -T postgres \
  psql -U ingestkit_admin -d ingestkit -c \
  "SELECT tenant_id,event_id,event_type,persisted_at FROM ingestkit.events ORDER BY persisted_at DESC LIMIT 10;"
~~~

A 202 response confirms the queue write. Give the background worker time to save
the rows before checking SQL. Sending the fixture again keeps one row per tenant
and event ID, with the first stored content unchanged. Use new UUIDs for new
business events. Keep the key in your backend, where users cannot retrieve it.

## Stop services or reload settings

~~~sh
# Stop and remove containers; preserve all named data volumes.
docker compose --env-file deploy/.env -f deploy/compose.yml down

# After an intentional configuration edit, recreate affected containers.
docker compose --env-file deploy/.env -f deploy/compose.yml up -d --build --wait
~~~

A plain `restart` keeps the old environment. Recreate affected containers after
editing configuration. Ordinary shutdown preserves database, broker and certificate
volumes; keep `down -v` out of that procedure.

Back up deploy/.env separately from the database. PostgreSQL uses its initial
admin password variable only when it creates the database directory; changing
that variable does not change an existing login's password.

## Disk durability settings

The [Caddy image](https://hub.docker.com/_/caddy) is pinned to `2.11.7-alpine`.
Redpanda starts outside dev-container mode, with one core, a 1 GiB allocation,
overprovisioned scheduling and `--unsafe-bypass-fsync=false`. Both topics use
`write.caching=false`. The [Redpanda mode reference](https://docs.redpanda.com/streaming/current/reference/rpk/rpk-redpanda/rpk-redpanda-mode/)
explains why development defaults need checking. PostgreSQL keeps its normal
fsync and synchronous-commit settings. None of these settings adds another copy
of the host's disks.

The [operator guide](../docs/operations.md) covers readiness, queue growth,
key rotation, backups, fresh-target restore and upgrades.
