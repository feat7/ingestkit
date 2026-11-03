# Delivery semantics

## Event contract

POST /v1/events with:
~~~json
{"event_id":"00000000-0000-4000-8000-000000000001","event_type":"order_completed","occurred_at":"2026-11-03T00:00:00Z","payload":{"amount":25}}
~~~

POST /v1/events/batch with {"events":[...envelopes...]}.
Each envelope requires a nonzero UUID, a 1–128-byte event type, an RFC 3339
occurred-at timestamp, and an object payload. The API supplies tenant and receipt
time. Unknown envelope fields, including a caller-supplied tenant, are rejected.
The API validates envelope structure; it does not validate business fields.

Limits: 256 KiB per event, 1 MiB per HTTP request, 100 events per HTTP batch.
Malformed input returns 400, invalid credentials 401, oversized input 413,
tenant rate limits 429, and unavailable/saturated publishing 503.

## Acknowledgement and retry

202 means all records in that response were acknowledged by the broker.
It does not mean the consumer or database has processed them.

The publish call is bounded to five seconds. A timeout or failed batch can be
ambiguous: some records may already exist in the broker. Retry using the same
event IDs. Batches are validated together but are not broker transactions.

PostgreSQL stores one row per tenant and UUID. First stored content wins;
reusing an ID for different content is a caller error. No payload equality check
is promised. Delivery can repeat; the storage effect is idempotent.

## Failures

| Failure | Behaviour |
| --- | --- |
| PostgreSQL unavailable | Keep source progress uncommitted and retry. |
| Database commit, then consumer crash | Replay; duplicates are ignored in storage. |
| Malformed broker envelope | Preserve raw bytes/key/headers/source offsets in the quarantine topic. |
| Quarantine unavailable | Leave the source pending; never commit past it. |
| Rebalance or failed commit | Rejoin from committed offsets; unfinished work can replay. |
| Shutdown deadline exceeded | Exit with failure rather than claim a successful drain. |

The consumer polls at most 500 records, has a fifteen-second processing/retry
window, and a ten-second group session timeout. Fetch byte settings guide
requests; they are not a hard process-memory ceiling.

## Durability assumptions

The local broker has one replica and seven-day retention. Both event and
quarantine topics explicitly disable write caching, including the development
container's relaxed default. PostgreSQL fsync and synchronous commit stay enabled.
These settings do not protect against loss of the only disk or an outage beyond
retention. Tenant key scoping is not database row-level isolation.

The database's persisted_at is an insertion timestamp, not proof of the
physical commit instant. Receipt-to-storage metrics are observed after transaction
completion and assume synchronized clocks on the local host.

## Inspect and repair quarantine

~~~sh
docker compose exec redpanda rpk topic consume ingestkit.dlq -n 1 -X brokers=redpanda:9092
~~~

The quarantine value contains base64 original bytes and source coordinates.
Decode and repair the envelope, then resubmit through the authenticated API with
the original event ID. Invalid IDs require an explicit operator choice of a new
ID. Never bulk-replay unchecked quarantine into the broker or skip source offsets
to hide a failure.
