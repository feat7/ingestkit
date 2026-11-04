# What happens after an event collector says "accepted"?

Suppose your checkout service has just saved a $25 order. It sends an
order-completed event to IngestKit and receives HTTP 202. PostgreSQL is offline
at the time. The event is waiting in Redpanda, the queue between the HTTP API
and the database worker.

The response confirms queue acceptance; storage is a separate step. A lost
response can cause the application to send the event again. A worker that stops
after saving the row can also read it again. Keeping the same event ID makes
both retries safe.

IngestKit is a small Go collector you can deploy with Docker Compose. The repo
also has Python and Node examples for sending events, and an order application
that keeps pending events in its database.

## Follow the event to PostgreSQL

Your backend gives the event a UUID, a unique ID that stays the same when it
retries. It also sends an event type, occurrence time and JSON-object payload.
The API key selects a tenant, the label stored with each event. Callers cannot
choose another tenant in the request body. The API adds the time it received the event.

<a href="assets/event-journey.svg"><picture><source media="(max-width:600px)" srcset="assets/event-journey-mobile.svg"><img src="assets/event-journey.svg" alt="The backend sends an event to the HTTPS endpoint. Redpanda holds it while the Go worker saves it in PostgreSQL."></picture></a>

Caddy receives HTTPS traffic and forwards it to the Go API. The API checks the
key and event, writes to Redpanda, and returns 202 after Redpanda acknowledges
all events named in the response. It never connects to PostgreSQL. The worker
reads the queue separately and writes rows in batches.

Unknown request fields and JSON that PostgreSQL cannot store are rejected before
entering the queue. Your application still validates its own business payload:
the collector does not decide whether an order amount makes sense.

The [deployment walkthrough](../deploy/README.md) starts with a fresh checkout,
generates private settings, sends three events and finds their IDs in SQL.
With your own hostname pointing at the server and ports 80/443 reachable, the
startup commands are:

~~~sh
deploy/configure.sh events.example.com
docker compose --env-file deploy/.env -f deploy/compose.yml up -d --build --wait
~~~

Docker builds Go inside the images. The host needs Docker/Compose, Git, OpenSSL
and curl. Only Caddy publishes ports; the API, queue, worker, database and metrics
stay on the private network. Migrations use an admin login and the worker uses
a restricted writer login. Named volumes keep the database, queue and certificates.
The setup script creates secrets in an ignored mode-0600 file, refuses to replace
an existing file, and ordinary shutdown preserves the volumes. Local tests use
a trusted private certificate authority; public DNS and ACME issuance were not
exercised by those tests.

## First, save the event in the application

A saved order does not by itself give you a saved event. If the application
commits the order and then crashes before the HTTP send, the collector has
nothing to recover. An in-memory retry loop disappears with the process.

The [transactional outbox pattern](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html)
puts the order and its pending event in one database transaction. They either
commit together or neither does. A separate dispatcher reads the pending events
and sends them to the collector.

The [SQLite order example](../examples/outbox/README.md) stores the UUID and exact
serialized event with the order. Creating the same order again with the same
amount returns the original event ID; a different amount is rejected. Run these commands from the configured checkout
with its initial generated key mapping. Use your hostname in API_URL.
On another backend host, supply the URL
and key through private application settings:

~~~sh
export API_URL=https://events.example.com
INGESTKIT_API_KEY=$(sed -n 's/^API_KEYS={"\([0-9a-f]*\)":"default"}/\1/p' deploy/.env)
export INGESTKIT_API_KEY
python3 -m examples.outbox create-order --database private/orders.db \
  --order-id order-123 --amount-cents 2500
python3 -m examples.outbox dispatch --database private/orders.db
python3 -m examples.outbox status --database private/orders.db
~~~

The dispatcher releases its SQLite connection before sending HTTP, so it holds
no business write lock during the network wait. It marks the event `acknowledged`
only after checking that the returned IDs match. A network or authentication
failure leaves it `pending`. A permanent 400/413 rejection stays `rejected` and visible for repair.
Acknowledged here means the queue accepted the event; PostgreSQL may still be
catching up.

The dispatcher can also crash after receiving 202 but before updating its local
row. On restart, that row still says pending. Sending its saved UUID and body
again is safe because the collector stores one row per tenant and event ID.
A test pauses a separate dispatcher process just after an actual response, kills
it and runs dispatch again. The database still contains one row.

The example runs one dispatcher at a time. Multiple dispatchers need a claim or
lease in the application's database. SQLite makes the pattern runnable here;
an application with an existing transactional database can put its outbox there.
The [Python and Node clients](integration.md) can also be used directly when the
application already retains the events it needs to retry. They have no package
dependencies, check the response IDs, retry the same serialized bytes and return
safe status/request-ID metadata. Keys belong on the backend.

## Then, save the row before saving the worker's place

The worker needs to remember how far it has read through each queue partition.
That saved position is called an offset. It determines where a replacement
worker starts after a crash.

For valid events, the worker does these operations in order:

1. Read a bounded group of records from a partition.
2. Save them with one multi-row INSERT and `ON CONFLICT DO NOTHING`.
3. Save the read position in Redpanda after the database commit succeeds.

The table's unique key is `(tenant_id,event_id)`. A crash between steps 2 and 3
leaves the row saved and the read position behind. The next worker reads that
event again. PostgreSQL sees the existing key, leaves the original row alone,
and the worker can then save its new position.

<a href="assets/worker-replay.svg"><picture><source media="(max-width:600px)" srcset="assets/worker-replay-mobile.svg"><img src="assets/worker-replay.svg" alt="The worker saves a row and stops before saving its queue position. After restart it reads the same event ID; PostgreSQL keeps the existing row."></picture></a>

The crash test stops a helper exactly after the real SQL commit, checks that the
queue position is still uncommitted, then starts a replacement in the same group.
It verifies both the replay and the single stored row. Random restarts alone
would not prove that this particular gap had been tested.

The HTTP response and worker processing can run at the same time after queue
acceptance; they have no single global packet order. The
[complete request sequence](assets/delivery.svg) and [compact guide](assets/delivery-mobile.svg)
show the separate acknowledgements. [Kafka's delivery discussion](https://kafka.apache.org/41/design/design/)
explains the wider issue: a Kafka transaction does not automatically include an
external PostgreSQL write. IngestKit reads events again and uses a unique database
key to handle that gap. It keeps the first stored content; reusing an ID for
different content is a caller error, not an update.

If a queue record is malformed, the worker saves its original bytes and source
details in a separate quarantine topic for inspection. It waits for that write
to succeed before moving past the bad record. A database outage is retried; it
does not turn valid events into quarantine records. The [delivery reference](delivery.md)
covers the input restrictions, request limits and recovery assumptions.

The API waits up to five seconds for publication and keeps buffering bounded.
A failed HTTP batch can still have put some records into the queue; validation
together does not make the batch one transaction across partitions or SQL writes.
Keep the original IDs and content after an uncertain response.

## One atomic database write per batch

The deployed writer builds a multirow INSERT with
`ON CONFLICT (tenant_id,event_id) DO NOTHING`. The transaction either saves its
new rows or rolls back; a replay keeps each existing row. The reference COPY
comparison includes creating a temporary table, loading it and merging its rows.
That is a different algorithm from direct COPY into the destination.

The current connection study does not compare writer methods. A writer
comparison needs the complete algorithm, transaction boundaries, duplicate
policy, payload size and actual batch sizes. Previous comparative bundles were
archived unchanged and withdrawn from public claims.

## Count the work at each boundary

A requested sending rate is not an achieved rate. Record planned events, requests
actually started, successful acknowledgements and independently verified rows.
A generator miss is work that was never sent. An uncertain response can follow
partial publication, so retry the same event IDs and keep unknown outcomes
separate from success.

Concurrent HTTP requests, open TCP connections, idle connection retention,
producer record buffers and database pool connections are different limits.
Measure their actual use and waits before attributing a throughput ceiling.
HTTP 202 acknowledges the broker; PostgreSQL may still be catching up.

The SQL observer must not block accounting for completed HTTP requests. Failed
observations make visibility timing unavailable; they must not erase a successful
independent ID/payload check or turn bookkeeping time into database throughput.
The [measurement guide](../benchmarks/README.md) describes the current audit.

## Check recovery separately from throughput

Use the recovery demo and owned integration tests to stop PostgreSQL, restart a
worker and exercise application outbox retry. Check accepted IDs and contents
after recovery. Readiness, broker offset lag and verified rows answer different
questions. A process starting is not proof that queued work has drained, and
an observation gap supplies no invented latency sample.

Other real-service tests revoke INSERT while the database still answers Ping,
then restore it and check processing readiness; rotate overlapping API keys; and
restore a backup into a separate empty deployment while keeping the source intact.
Worker logs report the failed operation, safe error class, SQLSTATE and queue
position. Repeated failures are throttled; raw errors, keys, database URLs and
payloads stay out of those messages. The [operator guide](operations.md) explains
how to check progress, rotate keys and restore data. After an older database
restore, replay must be planned explicitly: the queue's saved positions do not
rewind with the database, and only retained records can fill the gap.

## Where the results apply

The [audited connection study](../benchmarks/README.md) contains 36 ten-second
windows on a shared local machine. With 128 workers and four TCP connections,
connection acquisition averaged 873–969 ms. Eight connections reached roughly
15k acknowledged events/s. Larger caps sometimes introduced uncertain responses;
the producer reached its 1,000-record buffer limit. The sampled consumer pool
showed no new acquisition wait, so its limit stayed unchanged.

Offering 30k or 50k did not deliver those rates cleanly. Every acknowledged ID
and its content passed an independent SQL check, while unknown publications
stayed separate. The measurement guide shows all repetitions, payload sizes and
validity limits. These short windows establish no sustained maximum capacity.

The supported deployment is one host, one queue replica and finite retention.
Losing that host or its disks can lose events. Your application owns its business
schema and event IDs; you own backups, disk monitoring and SQL retention. If direct
PostgreSQL writes already meet your needs, a queue adds another service to operate.

For related designs, read [PostHog's capture and processing description](https://posthog.com/docs/how-posthog-works/ingestion-pipeline)
and [Segment's discussion of event-ID deduplication](https://www.twilio.com/en-us/blog/insights/exactly-once-delivery).
Their storage, scale and guarantees differ. The [deployment map](assets/deployment.svg)
and [compact map](assets/deployment-mobile.svg) show the services you would run
for this implementation.
