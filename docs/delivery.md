# Delivery semantics

The [HTTP reference](api.md) defines the request envelope, limits and responses.
The promise is broker acknowledgement followed by asynchronous, replay-safe
PostgreSQL storage. The application must retain stable IDs and content when
delivery is uncertain.

<a href="assets/delivery.svg"><picture><source media="(max-width:600px)" srcset="assets/delivery-mobile.svg"><img src="assets/delivery.svg" alt="The API acknowledges the broker; the consumer commits SQL before offsets; a crash between commits replays the same UUID."></picture></a>

[Full sequence](assets/delivery.svg) · [Figure sources](figures/README.md)

The lower sequence segment is an alternative crash path. HTTP response timing
and consumer processing are concurrent after broker acceptance; the figure is
not a global ordering of every network packet.

## Three boundaries

| Boundary | What it establishes | What remains pending |
| --- | --- | --- |
| HTTP 202 with matching IDs | The broker acknowledged all response records | Consumer processing and PostgreSQL storage |
| SQL statement commits | A valid consumer segment stored atomically; conflicting IDs kept their first stored content | Source-offset commit |
| Source offsets commit | Successfully handled source records can be skipped by this consumer group | Later application processing and operator retention/backup policy |

The publisher waits at most five seconds and uses bounded buffers. A timeout,
503 or interrupted response can be ambiguous: some records may already exist in
the log. A validated HTTP batch is not a broker transaction and can span several
partitions/SQL segments. Retry the same UUIDs and content. The supplied clients
check the complete acknowledgement rather than assuming any 202 is success.

PostgreSQL has a unique `(tenant_id,event_id)` key. Replaying a segment after SQL
commit and before offset commit cannot create another row. Reusing an ID with
different content is a caller error: conflicts are ignored, not compared or
updated. Tenant scoping at HTTP is not database row-level isolation. There is no
transaction spanning Kafka and PostgreSQL or unconditional exactly-once claim.

A separate [application outbox](../examples/outbox/README.md) saves business state
and an event in one local transaction. Its acknowledged state means broker ACK,
not PostgreSQL persistence. A crash after HTTP ACK but before marking the outbox
row leaves that event pending; a new dispatcher resends its original bytes/UUID.

## Failure behavior

| Failure | Behavior |
| --- | --- |
| PostgreSQL unavailable or writes denied | Retry; keep affected source progress uncommitted |
| SQL commit followed by consumer crash | Replay the same IDs; ignore storage duplicates |
| Malformed broker envelope | Preserve original bytes/key/headers/source coordinates in the separate quarantine topic |
| Quarantine unavailable | Do not advance past that record |
| Rebalance or offset-commit failure | Rejoin from committed offsets; unfinished work may replay |
| Broker unavailable | HTTP may fail uncertainly; application outbox retains pending events |
| Shutdown deadline exceeded | Exit with failure; do not claim a successful drain |

The consumer polls at most 500 records and processes each partition sequentially.
Its processing/retry window is fifteen seconds; its group session timeout is ten
seconds. Source progress follows successful storage or acknowledged quarantine.
Fetch-byte settings guide requests; they are not a hard process-memory ceiling.
Shutdown stops new polls and gives current work ten seconds; it does not drain
the whole retained log.

Consumer readiness includes failed storage, quarantine and offset operations.
Connectivity alone cannot clear a processing failure. After fixing the cause,
the matching operation must succeed (or a process restart starts fresh state).
Use [safe diagnostics and progress checks](operations.md), not API readiness
alone, to assess a stuck consumer.

## Durability and observation

The deployment uses one broker replica and seven-day event/quarantine retention.
Topic `write.caching=false` and the actual broker process's
`--unsafe-bypass-fsync=false` are both verified. PostgreSQL fsync and synchronous
commit remain enabled. These are configuration checks, not a physical power-loss
experiment or protection from losing the only host/disk. An outage beyond retained
data can leave an unrecoverable gap. PostgreSQL rows have no automatic expiry.
Restoring an older database does not rewind broker offsets; see the explicit
[backup/restore and retained-log replay procedure](operations.md).

`persisted_at` is an insertion timestamp, not an exact physical commit clock.
The study records API receipt to the first completed SQL observation of a
committed row, including polling/query overhead. Receipt and SQL observation use
the deployment clock; host monotonic timing schedules requests. Final ID/content
verification is independent. An observer gap cannot provide exact commit times.

## Representable payloads

The API validates structure and PostgreSQL JSONB representation, not business
schema. Payloads must be objects, valid UTF-8 with paired Unicode surrogate
escapes, no NUL in strings/keys, and numeric representations within 131,072 integer
digits and 16,383 fractional digits. The root counts as level one of a conservative
64-level nesting bound. Envelope text and tenant IDs also reject NUL and invalid
Unicode. These checks run before HTTP publication and on broker decoding; an
incompatible broker record is quarantined rather than retried as a valid event.
See [PostgreSQL JSON restrictions](https://www.postgresql.org/docs/18/datatype-json.html).

For private inspection, define the deployment `dc` helper from the operator guide:

~~~sh
dc exec -T redpanda rpk topic consume ingestkit.dlq -n 1 -X brokers=redpanda:9092
~~~

Quarantine contains data, including base64 original bytes. Inspect privately,
repair the envelope and resubmit through the authenticated API with its original
ID. If the original ID is invalid, choose a new one deliberately. Do not bulk
replay unchecked quarantine or skip source offsets to hide an error.
