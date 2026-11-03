# Building an event pipeline in Go—and testing what survives a crash

An HTTP 202 response is easy to produce. Explaining what it means after a
process dies is harder.

I wanted IngestKit to make that boundary visible. The useful artifact is a small
collector, a reproducible workload and failure tests that reconcile event IDs.
The implementation sends JSON events through a Go API to Redpanda, then a Go
consumer writes them into PostgreSQL. It also produced an inconvenient benchmark
result: at the tested small batch size, ordinary batch INSERT beat the COPY path.

These are newly performed local experiments, not a production success story or
a historical sequence of optimizations. The measured code is
[deefc8162504fa482c19c63edda05c1c1a4c2ae7](https://github.com/feat7/ingestkit/tree/deefc8162504fa482c19c63edda05c1c1a4c2ae7).
[Raw results and environment manifests](../benchmarks/results/local-2026-11-03/README.md)
are part of the repository.

## Start with the promise

A caller supplies an event UUID, event type, occurrence timestamp and JSON object.
The API key determines the tenant; the caller cannot choose another tenant in
the envelope. The API supplies a receipt timestamp and validates the complete
request before publishing it.

~~~json
{"event_id":"00000000-0000-4000-8000-000000000003","event_type":"order_completed","occurred_at":"2026-11-03T00:00:00Z","payload":{"order_id":"order-1","amount":25}}
~~~

The API returns 202 only after the configured broker acknowledges every event
covered by the response. It does not wait for PostgreSQL. That distinction lets
the service accept events during a database outage, within the broker's finite
retention and durability assumptions.

There is an awkward case: the request times out after some records reach the
broker. Returning an error does not undo those records. Clients therefore keep
the same IDs when retrying uncertain requests. A batch is validated together;
it is not an atomic broker transaction. The publisher has a five-second deadline
and bounded buffering, so a failing dependency cannot create an unlimited queue
inside the API process.

## Put the crash window in the design

The consumer follows a short sequence:

1. Read a bounded batch from a partition.
2. COPY valid events into a temporary staging table.
3. Merge into the destination with `ON CONFLICT DO NOTHING`, then commit PostgreSQL.
4. Commit the successfully handled broker offsets.

The primary key is `(tenant_id,event_id)`. If the consumer dies between steps
three and four, the database contains the event while the broker still considers
it pending. Restarting replays it; the unique key ignores the extra insertion.

This is repeatable delivery with an idempotent storage effect. It is not a
transaction spanning Kafka and PostgreSQL. First stored content wins, so reusing
an ID for different content is a caller error. The implementation does not
compare payloads on a conflict.

~~~mermaid
sequenceDiagram
  participant C as Consumer
  participant P as PostgreSQL
  participant B as Broker
  C->>P: Stage + merge
  P-->>C: Transaction committed
  Note over C: Kill here: row exists, offset pending
  C->>B: Commit source offset
  Note over C,B: Restart can replay, unique key prevents another row
~~~

Malformed broker envelopes take another path: preserve their raw bytes, key,
headers and source coordinates in a quarantine topic. The source offset advances
only after that write is acknowledged. If quarantine fails, progress stops at
that record. A database outage is retried; it is not grounds for discarding a
valid event. Processing remains sequential so that a later offset cannot skip
unfinished work in the same partition.

## Test the boundary instead of hoping to hit it

The recovery suite uses real isolated PostgreSQL and Redpanda containers. The
crash test wraps the real store and pauses immediately after the database commit,
before the consumer can commit its offset. The parent kills only that helper
process, verifies the source offset is still pending, and starts a replacement
consumer in the same group. The replacement observes a duplicate and advances
the offset; the database still contains one row.

Other tests stop PostgreSQL while events are acknowledged, restore it and
reconcile the accepted IDs. They also stop the broker, fail quarantine writes,
introduce a second consumer to cause a rebalance, and exercise graceful and forced
shutdown. One deliberate negative control temporarily committed offsets before
storage; the ordering regression test failed, then passed after restoring the
safe order. This is not exhaustive mutation testing, but it proves the ordering
check can detect the mistake it is meant to prevent.

Each run owns distinct ports, volumes and a Compose project. Cleanup verifies
ownership. A failed experiment saves evidence rather than silently resetting a
shared database. A live outage walkthrough is available as `make demo-recovery`.

## COPY was not the winner here

The database experiment compares single INSERT, batch INSERT, and the actual
COPY/staging/merge store. All target the same indexed table with the same conflict
policy and durability settings. Workload generation, warm-up, dataset reset and
exact ID/payload verification happen outside the timer. Method order rotates
across three measured repetitions.

For 10,000 unique submissions with 256-byte JSON payloads and batches of 100:

| Method | Median elapsed | Submitted events/s |
| --- | ---: | ---: |
| Single INSERT | 16.909s | 591 |
| Batch INSERT | 0.492s | 20,333 |
| COPY/staging/merge | 1.011s | 9,889 |

Single INSERT commits every row; the other methods commit every batch. That
commit amortization is part of this practical comparison, not an isolated test
of SQL parser cost. The COPY path additionally creates a temporary table and
merges into an indexed target. Measuring plain COPY into an empty unindexed table
would answer a different question from this service's replay-safe writes.

With 10% duplicate submissions, each method inserted exactly 9,000 distinct IDs.
Median elapsed times were 13.054s, 0.256s and 0.706s respectively. Those runs did
less insertion work and used a separate local stack. They demonstrate correct
duplicate handling and its total workload timing, not a clean percentage cost
for deduplication. The raw repetitions show substantial timing variation.

The result gives me no evidence to promote COPY/staging as the fastest small-batch
writer. The reference keeps it visible as an experiment with replay semantics;
ordinary batched SQL is the stronger candidate for this measured workload.
A practical deployment should benchmark its expected batch sizes and payloads
before choosing. A generic JSONB envelope also avoids generated per-event tables,
but I have not measured its storage or query overhead against typed schemas.

## Measure the whole path separately

Database throughput is not collector throughput. The end-to-end driver offers
10,000 unique events per ten-second window, in HTTP batches of 100 with eight
workers. After a two-second warm-up, it runs three repetitions of each pattern:
steady at 1,000 events/s, and a burst of 4,000 events/s for the first 2.5 seconds
followed by idle time.

All six windows accepted and stored every ID, with no generator queue misses,
late schedules, rejected requests or ambiguous deliveries. Steady HTTP ACK p95
was 27.4–31.0ms. Receipt-to-visible-storage p95 was 72.7–85.2ms.

Burst results were more variable: ACK p95 was 137.9ms in the first repetition,
then 18.1ms and 17.0ms. Storage visibility p95 was 261.5ms, 68.7ms and 68.8ms.
The coarse observed backlog peaked at 881 events in the first burst. Its cause
is not established. Reporting only the best repetition would conceal a useful
limitation of this evidence.

![All database and pipeline repetitions](assets/experiments.svg)

Storage visibility is deliberately labelled an upper bound: it runs from API
receipt to the end of the first SQL query that sees the committed row. It includes
50ms polling and query delay. The database's `persisted_at` default is an insertion
timestamp, not a physical commit clock. HTTP latency is per successful request;
storage latency is per observed event. Those sample populations are different.

Complete-window stored throughput was approximately 995–996 unique events/s,
including final confirmation. The burst's idle time is included. This verifies
a load point; it does not establish maximum capacity, active-burst processing
capacity or long-term latency guarantees.

## What the experiment does not establish

Everything ran on a shared Apple M1 Max with OrbStack. The host generator and SQL
observer compete with the services. PostgreSQL had fsync, synchronous commit and
full-page writes enabled. The single Redpanda broker had three event partitions,
one replica, seven-day retention and write caching explicitly disabled. Image
digests, runtime settings and coarse resource snapshots are saved with the data.

A single broker does not survive loss of its only disk. An outage beyond retention
can expire pending events. These tests do not demonstrate host-loss recovery,
maximum throughput, cloud cost, deployment security or production adoption. API
key scoping is also not database row-level isolation.

For small workloads, direct batched PostgreSQL writes may be the better design.
The broker buys outage buffering and replay, while adding another service to run.
Generated SDKs and a schema compiler are not necessary to exercise this contract:
the Python and Node examples are plain HTTP clients. Business payload validation
belongs to the sending application.

## Reproduce and inspect

With Go and Docker installed, run `make demo`, `make demo-recovery` and
`make test-integration`. The [benchmark instructions](../benchmarks/README.md)
include the exact measurement commands. Every acknowledged ID is reconciled
before a run succeeds; each experiment prints its evidence location.

Useful related reading: Tiger Data's [PostgreSQL batch ingest comparison](https://www.tigerdata.com/blog/benchmarking-postgresql-batch-ingest)
examines how method and batch size affect results. Segment's
[deduplication write-up](https://www.twilio.com/en-us/blog/insights/exactly-once-delivery)
explores stable message IDs at a much larger scale. PostHog's
[ingestion documentation](https://posthog.com/docs/how-posthog-works/ingestion-pipeline)
shows a capture boundary feeding downstream processing through Kafka. These are
references for reasoning about the problem, not evidence that this project
operates at their scale.
