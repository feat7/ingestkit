# What "accepted" and "saved" mean

The application, queue and database finish their work at different times. HTTP
202 confirms that Redpanda accepted the event. A background worker saves it in
PostgreSQL afterward. Keep the original event ID and content whenever you are
unsure whether a request arrived. The [HTTP reference](api.md) lists the request
fields, limits and responses. The API key selects the tenant label; event IDs
are unique within that tenant.

## What happens if the worker stops after writing?

The worker saves the event first, then saves its position in the queue. That
position is called an offset. It tells the next worker where to resume.

<a href="assets/worker-replay.svg"><picture><source media="(max-width:600px)" srcset="assets/worker-replay-mobile.svg"><img src="assets/worker-replay.svg" alt="The row commits before the queue position. A restarted worker reads the same ID again and PostgreSQL keeps one row."></picture></a>

If the process stops between those operations, the next worker reads the same
event again. PostgreSQL's unique `(tenant_id,event_id)` key ignores the repeated
insert. The first stored content stays unchanged. Sending different content
with the same ID is a caller error; the database does not compare or update it.

The [full HTTP sequence](assets/delivery.svg), [compact sequence](assets/delivery-mobile.svg)
and [figure sources](figures/README.md) give the wider request path. HTTP response
timing and worker processing can overlap after queue acceptance. The lower segment
in the full sequence is an alternative crash path, not a global packet order.

## What each completed operation tells you

| Completed operation | What is known | What may still need to happen |
| --- | --- | --- |
| HTTP 202 with matching IDs | The queue acknowledged every record named in the response | The worker saves the events to PostgreSQL |
| SQL statement commits | The valid segment is saved atomically; repeated IDs keep their first content | The worker saves its queue position |
| Source offsets commit | This consumer group can resume past the handled records | Later application processing, retention and backups |

The publisher waits at most five seconds and keeps buffering bounded. A timeout,
503 or interrupted response may occur after some records reached the queue.
The API validates an HTTP batch together, but publication is not a broker
transaction and the batch can span partitions and SQL segments. Resend the same
UUIDs and content. The example clients check all returned IDs before calling a
request successful.

The unique key makes repeated storage safe. It does not create one transaction
across Kafka and PostgreSQL, or an unconditional exactly-once guarantee. HTTP
checks the tenant selected by the key; PostgreSQL row-level isolation is separate.

An [application outbox](../examples/outbox/README.md) covers an earlier gap: it
saves the business change and pending event in one local transaction. If its
dispatcher stops after receiving 202 but before marking the local row, the event
stays pending. The next process resends the original UUID and bytes. An
acknowledged outbox row confirms queue acceptance, not the PostgreSQL write.

## How failures are handled

| Failure | What the collector does |
| --- | --- |
| PostgreSQL is unavailable or denies writes | Retries and keeps the affected queue position uncommitted |
| Worker crashes after SQL commit | Reads the same IDs again; PostgreSQL ignores the duplicate inserts |
| Broker record has an invalid envelope | Saves its original bytes/key/headers and source coordinates in the quarantine topic |
| Quarantine write fails | Keeps the source position before that record |
| Consumer group rebalances or an offset commit fails | Resumes from committed offsets; unfinished work may be read again |
| Broker is unavailable | HTTP may fail with an uncertain outcome; the application outbox keeps its pending events |
| Shutdown runs out of time | Exits with failure instead of reporting a completed drain |

The worker polls at most 500 records and processes each partition sequentially.
It has fifteen seconds for a processing/retry cycle and a ten-second group
session timeout. It advances only after storage or acknowledged quarantine.
Fetch-byte settings guide broker requests; they do not cap total process memory.
Shutdown stops new polls and allows ten seconds for current work, not the whole
retained queue.

Consumer readiness tracks failed storage, quarantine and offset operations.
A successful connectivity check cannot clear one of those failures. After the
cause is fixed, the failed operation must succeed; restarting the process starts
fresh health state. Check [diagnostics and actual progress](operations.md), since
API readiness alone cannot tell you whether database storage is moving.

## How long recovery remains possible

The deployment has one broker replica. Event and quarantine records have seven-day
retention. Both topic `write.caching=false` and the actual process flag
`--unsafe-bypass-fsync=false` are verified. PostgreSQL fsync and synchronous
commit stay enabled. These checks establish the configuration; they are not a
physical power-loss experiment. Losing the only host or disk can lose data, and
an outage longer than retained history can leave a gap that cannot be recovered.
PostgreSQL rows have no automatic expiry.

An older database restore does not rewind the queue's saved offsets. Plan replay
from retained data using the [restore procedure](operations.md).

The study's `persisted_at` is an insertion timestamp, not an exact commit clock.
Storage timing starts when the API receives an event and ends when a completed
SQL query first sees its committed row. It includes polling/query delay. Receipt
and SQL observation use the deployment clock; the host uses monotonic time for
request scheduling. A final independent query checks IDs and content. No exact
commit time can be recovered from a gap in observation.

## JSON the database can store

The API checks event structure and PostgreSQL JSONB limits. Your application
checks the business schema. A payload must be a JSON object with valid UTF-8,
paired Unicode surrogate escapes and no NUL (zero character) in strings or keys. Numbers must
fit within 131,072 integer digits and 16,383 fractional digits. The nesting limit
is 64 levels, counting the root as one. Envelope text and tenant IDs also reject
NUL and invalid Unicode.

The same checks run before HTTP publication and when reading broker records.
An incompatible broker record goes to quarantine instead of blocking the queue
as a repeatedly failing valid event. See [PostgreSQL's JSON restrictions](https://www.postgresql.org/docs/18/datatype-json.html).

For private inspection, first define the deployment `dc` helper from the operator
guide, then run:

~~~sh
dc exec -T redpanda rpk topic consume ingestkit.dlq -n 1 -X brokers=redpanda:9092
~~~

Quarantine includes event data and base64-encoded original bytes. Inspect it
privately, repair the envelope and send it through the authenticated API with
the original ID. Choose a new ID deliberately if the original is invalid.
Do not replay unreviewed records in bulk or skip offsets to hide a failure.
