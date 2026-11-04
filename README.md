# IngestKit

Collect events from your backend and save them in PostgreSQL. A queue in between
lets IngestKit keep accepting events during a temporary database outage.

For example, your checkout service sends an `order_completed` event with a saved
ID. Redpanda holds it in the queue until a background worker writes it to
PostgreSQL. If the request times out, resend the same ID; the database keeps one
row for that event. The API key selects the tenant, a label stored with your events.

<a href="docs/assets/event-journey.svg"><picture><source media="(max-width:600px)" srcset="docs/assets/event-journey-mobile.svg"><img src="docs/assets/event-journey.svg" alt="An application sends an event over HTTPS. Redpanda holds it until a background worker writes it to PostgreSQL."></picture></a>

HTTP 202 means the queue accepted the event. The database write happens
separately. [Read the story of an event, an outage and a retry](docs/article.md).

## Run it on your server

You need a Linux host with Git, Docker/Compose 2.24.4+, OpenSSL and curl.
Point a hostname you own at the server and allow traffic on ports 80 and 443.
Replace `events.example.com` below with that hostname. Docker builds the Go
programs, so you do not need to install Go on the server.

~~~sh
git clone https://github.com/feat7/ingestkit.git
cd ingestkit
deploy/configure.sh events.example.com
docker compose --env-file deploy/.env -f deploy/compose.yml up -d --build --wait
~~~

The setup script writes fresh secrets to ignored `deploy/.env` with file mode
0600, so only its owner can read it. It refuses to overwrite existing settings.
Caddy handles HTTPS; the API, queue and database have no public ports. The
[deployment guide](deploy/README.md) covers resources, DNS and certificates.
Public certificates need working DNS and reachable ports; automated tests use
a local certificate authority that the test client explicitly trusts.

## Send an event and find it in the database

For the initial generated configuration, load the API key without printing it
and send the three checkout events included in the repository:

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

Give the worker time to save the events before running the SQL query. Sending
this fixture again still leaves three events for the initial tenant. For new
business events, create new IDs. The database key is `(tenant_id,event_id)`:
your API key selects the tenant, and your application supplies the event ID.
The first content saved for that key stays in the table.

Stop the services with this command. Docker keeps their named data volumes:

~~~sh
docker compose --env-file deploy/.env -f deploy/compose.yml down
~~~

## Send events from your application

The [Python and Node examples](docs/integration.md) are small modules you can
copy into a backend. They need no third-party packages. Keep the API key on
your server and save each event's ID and content before sending it.

For events that must survive an application crash, try the
[order and outbox example](examples/outbox/README.md). It saves an order and a
pending event in the same database transaction. A separate command sends the
pending events and can pick up where a previous process stopped. The
[API reference](docs/api.md) explains request limits and which failures to retry.
The [operator guide](docs/operations.md) covers backups, key rotation and a queue
that stops making progress.

## Try an outage locally

With Docker and Go 1.24+ installed, run:

~~~sh
make demo DEMO_FLAGS=-cleanup
make demo-recovery DEMO_FLAGS=-cleanup
~~~

The second demo stops PostgreSQL, sends events, restarts the database and checks
that the same IDs reached the table. The [experiment results](benchmarks/results/selfhost-2026-11-04/README.md)
go further: 25 configurations, three runs each, with saved samples and explicit
limits. The database comparison led to a simpler writer: batch INSERT had a lower
median time than COPY/staging/merge in all 11 tested database configurations.
Unknown delivery outcomes remain unknown in the reports; these runs do not
establish maximum capacity.

[Development and tests](docs/development.md) · [Run the experiments](benchmarks/README.md) ·
[Delivery details](docs/delivery.md) · [How the components fit together](docs/architecture.md)

This deployment runs on one host with one Redpanda replica and keeps queue
records for seven days by default.
Losing the host or its disks can lose data. An outage longer than the queue's
retention can leave events unrecoverable. PostgreSQL rows do not expire
automatically; plan backups, disk monitoring and database retention for your use.
