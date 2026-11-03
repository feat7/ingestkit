# Self-hosted deployment

Prepare private configuration for the hostname you will use:

~~~sh
deploy/configure.sh events.example.com
~~~

The script requires a POSIX shell, standard system utilities and OpenSSL. It
creates deploy/.env with mode 0600, three independent secrets and an initial API
key mapped to tenant default. It does not install software or contact a server.
Existing configuration is preserved; re-running against the same path fails.
Keep the file private and out of source control.

An optional second argument selects a configuration directory. localhost is
accepted for local TLS verification. Public deployment will require a hostname
pointing to the host and reachable gateway ports 80/443.

## Start on a server

Use a Linux host with Git, Docker Engine and Docker Compose v2.24.4 or newer,
OpenSSL, and curl. Allow room for the broker's 1 GiB allocation plus PostgreSQL,
the Go services and Docker; start with at least 4 GiB RAM and two CPUs available.
Disk requirements depend on traffic and retention. This is a single-host setup:
losing its disk can lose both queued and stored events.

Point the hostname's DNS records at the host. Ports 80 and 443 must be reachable
for Caddy's certificate issuance and HTTPS traffic. Caddy keeps its certificates
in a named volume. Public certificate issuance requires working DNS and external
reachability; the automated local test uses a private CA instead.

~~~sh
git clone https://github.com/feat7/ingestkit.git
cd ingestkit
deploy/configure.sh events.example.com
docker compose --env-file deploy/.env -f deploy/compose.yml config --quiet
docker compose --env-file deploy/.env -f deploy/compose.yml up -d --build --wait
~~~

Only the gateway publishes ports. PostgreSQL, Kafka, broker administration and
service metrics stay on Docker's private network. The consumer uses an
`ingestkit_writer` login with schema usage, SELECT and INSERT (no temporary-table privilege);
migrations use the separate admin login.

## Send and verify

For the initial generated mapping, load the API key locally without printing it:

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

HTTP 202 acknowledges the broker write. Storage happens asynchronously; allow
time for the consumer before checking SQL. Repeating the same IDs stores one row
per tenant and event ID. The first stored content wins. Use new UUIDs for new
business events. The API key belongs in trusted backend code.

## Stop or update configuration

~~~sh
# Stop and remove containers; preserve all named data volumes.
docker compose --env-file deploy/.env -f deploy/compose.yml down

# After an intentional configuration edit, recreate affected containers.
docker compose --env-file deploy/.env -f deploy/compose.yml up -d --build --wait
~~~

A plain `restart` does not reload environment values. Never add `down -v` to an
ordinary shutdown. Keep a private backup of deploy/.env as well as your database
backups: changing PostgreSQL's initial admin password variable does not change
the password inside an existing database.

The pinned [Caddy official image](https://hub.docker.com/_/caddy) is
`2.11.7-alpine`. Redpanda starts without dev-container mode, with a 1 GiB memory
allocation, one core, overprovisioned scheduling and explicit
`--unsafe-bypass-fsync=false`; both topics have `write.caching=false`. See the
[Redpanda mode reference](https://docs.redpanda.com/streaming/current/reference/rpk/rpk-redpanda/rpk-redpanda-mode/)
for why development defaults must be examined. PostgreSQL retains its normal
fsync and synchronous commit settings. These settings do not provide replication
or protection from losing this host.

See [operating the collector](../docs/operations.md) for readiness, disk growth,
key rotation, backup/restore and upgrade procedures.
