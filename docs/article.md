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

## The database comparison removed some code

The original writer used COPY, PostgreSQL's bulk-loading mechanism. Its complete
path created a temporary table, copied the events into it, merged them into the
indexed event table with duplicate handling, and committed. That is more work
than a plain COPY into an empty table.

[PostgreSQL's bulk-loading guidance](https://www.postgresql.org/docs/18/populate.html)
explains the cost of committing each row and the use of COPY for large loads.
The [recorded study](../benchmarks/results/selfhost-2026-11-04/README.md) used clean
source [213e4cc](../benchmarks/results/selfhost-2026-11-04/manifest-1.json) and measured
25 configurations three times each. The database comparison covered these complete implementations: one INSERT per
row, one INSERT per batch, and COPY followed by a merge.

Each method submitted 5,000 events against the same table, indexes and duplicate
policy, with database durability enabled. Input generation, reset, warm-up and
verification happened outside the measured write time. Method order rotated
across three runs. The single-row method committed every row; the other two
committed every batch.

For 256-byte payloads, batch size 100 and no duplicate IDs, the times were:

| Method | Median (s) | Range (s) |
| --- | ---: | ---: |
| Single INSERT | 6.510 | 4.710 to 8.375 |
| Batch INSERT | 0.155 | 0.121 to 0.247 |
| COPY/staging/merge | 0.486 | 0.212 to 0.523 |

The median is the middle of the three runs; the range shows all of them.
Batch INSERT also had a lower median than COPY/merge in all 11 database
configurations in both complete studies. The sweep covered batches from 1 to
1,000, payloads up to 16 KiB, and 10%, 50% and 90% repeated submissions. Some
individual runs overlapped or reversed the ordering. Repeated-ID cases created
fewer new rows, so their submitted-events rate measures different work.

The deployed worker now uses an atomic batched INSERT. Its rollback, duplicate,
restricted-role and exact crash-replay tests still pass, and it no longer needs
permission to create temporary tables. The original COPY transaction remains
an independent benchmark method. A second complete study measured the changed
writer from committed code.

These runs establish a choice for this workload. They do not locate a universal
batch size where COPY wins, compare UNNEST, or compare JSONB with typed tables.
[Tiger Data's batch-ingest comparison](https://www.tigerdata.com/blog/benchmarking-postgresql-batch-ingest)
uses other methods and sizes; its numbers cannot be transferred to this workload.
The [full database plots](assets/database-batching.svg), including their
[phone layout](assets/database-batching-mobile.svg), retain every measured run.

## Count what was sent, accepted and found

The HTTPS baseline tried to send 1,000 events per second for ten seconds: batches
of 100, 32 clients and 256-byte payloads. After two seconds of warm-up, all three
runs sent, received acknowledgements for, and found the stored content of 10,000
events each, without reported limits.

The response and storage measurements answer different questions. Across those
runs, HTTP response p95 was 40.19 to 42.92 ms: 95% of successful requests completed
within each run's p95. Receipt-to-observed-storage p95 was 101.61 to 103.08 ms,
with one sample per observed event. SQL checks ran at a target interval of 50 ms,
so that second measure includes waiting for the next query and the query itself.
It is an upper bound on when storage became visible.

The suite has 11 HTTP configurations, each run three times. Larger payloads use
batches of 10 to fit the request limit. Some configurations missed the sending
schedule or finished requests after the ten-second window. At 10,000 offered
events/s, run 2 left 1,000 IDs with uncertain HTTP outcomes absent at the final
check. The 16 KiB case left 160 in its second run. Those IDs remain unknown:
the report neither counts them as acknowledged success nor proves they could
never arrive later. An application should keep them for retry.

A missed sending schedule is work the client did not submit. An acknowledgement
just beyond a window boundary can happen without proving the server is saturated.
The [full HTTP plots](assets/pipeline-load.svg) and [phone layout](assets/pipeline-load-mobile.svg)
keep those cases and their limits. These short local runs do not establish
maximum capacity.

Rates also need a clear end time. The in-window acknowledgement rate counts only
responses received inside the target window. The verified storage rate includes
the window, unfinished requests, observation time and final verification.
Counting late rows against only the earlier window would inflate that rate.

To measure storage, the observer tracks the IDs it has not yet seen in a temporary
table on its own database connection. A transaction that commits late therefore
still gets checked. Scanning only an insertion timestamp such as `persisted_at`
could miss it, because insertion and commit happen at different times. A final,
independent query checks IDs and content. Rows found only then get no invented
latency sample. API receipt and SQL observation use the deployment clock; the
host schedules requests with a monotonic clock, which does not jump with wall time.

## Stop the dependencies and check the rows

The three recovery experiments each ran three times:

| What stops | When | What was checked afterward |
| --- | --- | --- |
| PostgreSQL | Stop requested at 3s, start at 8s, during 15s of offered 500 events/s | Every acknowledged ID and its content reached storage |
| Worker | KILL at 3s, restart requested at 5s, with the same offered load | Every acknowledged ID and its content reached storage after the group recovered |
| Redpanda queue | Stopped for eight seconds after stop completion; 100 outbox events; a failed attempt longer than the five-second publish deadline | A new dispatcher resent saved events; each run finished with 100 accepted and stored unique IDs |

The [recovery plots](assets/recovery.svg) and [phone layout](assets/recovery-mobile.svg)
show all runs and actual control times. Gaps stay visible when SQL cannot be
observed. A start command completing does not mean the worker has caught up,
and these tests do not replace the exact crash-window test.

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

The tests ran on an M1 Max host with an OrbStack Linux Docker VM. The VM exposed
10 CPUs and about 16 GiB RAM; Redpanda had one core and 1 GiB. The sender and SQL
observer shared the machine with existing workloads. Coarse container snapshots
cannot establish the sole bottleneck or a cloud-cost estimate.

The manifests record image IDs/digests, the actual Redpanda fsync-bypass flag, topic
write caching, database durability and verified HTTPS. An earlier development
capture had process-level fsync bypass enabled even with topic write caching off.
Its [historical correction](../benchmarks/results/local-2026-11-03/README.md) remains
with the original samples. The corrected studies use new measurements.

The supported deployment is one host, one queue replica and finite retention.
Losing that host or its disks can lose events. Your application owns its business
schema and event IDs; you own backups, disk monitoring and SQL retention. If direct
PostgreSQL writes already meet your needs, a queue adds another service to operate.

For related designs, read [PostHog's capture and processing description](https://posthog.com/docs/how-posthog-works/ingestion-pipeline)
and [Segment's discussion of event-ID deduplication](https://www.twilio.com/en-us/blog/insights/exactly-once-delivery).
Their storage, scale and guarantees differ. The [deployment map](assets/deployment.svg)
and [compact map](assets/deployment-mobile.svg) show the services you would run
for this implementation.
