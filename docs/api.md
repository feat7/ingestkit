# HTTP event contract

Use the HTTPS URL from [deployment](../deploy/README.md). Authenticate every
write with `Authorization: Bearer <key>` and send `Content-Type: application/json`.
The server maps the key to a tenant; callers do not supply tenant identity. The
deployment default token bucket refills at 10,000 **requests**/s per tenant,
with a 10,000-request burst allowance. These are configuration limits, not
measured capacity; batched events consume one request.

`POST /v1/events` accepts one envelope:

~~~json
{
  "event_id": "dc12060f-f3a6-4ed3-9ea9-2d396b260b41",
  "event_type": "order_completed",
  "occurred_at": "2026-11-04T00:00:00Z",
  "payload": {"order_id": "order-123", "amount_cents": 2500}
}
~~~

`POST /v1/events/batch` accepts `{ "events": [ ... ] }`. Use stable UUIDs that
were saved with the business operation. Create a new UUID for a new event, and
reuse the same UUID and serialized content for an uncertain retry. The first
stored event for `(tenant_id,event_id)` wins; a later payload does not update it.
Unknown envelope fields, malformed JSON and values incompatible with PostgreSQL
JSONB are rejected. Payloads must be objects. Event types are 1–128 bytes.
Timestamps must parse as RFC 3339. See [delivery details](delivery.md) for edge cases.

| Limit | Value |
| --- | --- |
| Encoded individual input event | 256 KiB |
| HTTP request body | 1 MiB |
| Events in a batch | 1–100 |
| Payload nesting | At most 64 levels |

Large events may require smaller batches to fit the request limit. For example,
ten 16 KiB payloads fit; one hundred do not.

## Responses and retries

A successful response is HTTP 202:

~~~json
{"status":"accepted","event_ids":["dc12060f-f3a6-4ed3-9ea9-2d396b260b41"],"request_id":"f8c22d85-5e93-4cb5-a456-18b0f104bcca"}
~~~

The returned IDs match input order. This acknowledges the broker append. It does
not wait for the consumer or PostgreSQL. A batch is not a transaction across HTTP,
the broker and database. A timed-out/failed request may have delivered some or all
records, so retry identical IDs and content rather than generating replacements.

| HTTP result | Caller action |
| --- | --- |
| 202 with matching IDs | Broker acknowledged; verify persistence separately when required |
| 400 | Fix the invalid envelope; do not blindly retry |
| 401 | Fix/rotate authentication; preserve unsent business events |
| 404 or redirect | Correct the endpoint/configuration |
| 413 | Reduce event/request size; split oversized batches |
| 429 | Retry the same content with bounded backoff |
| 500/502/503 or other server failure | Delivery may be uncertain; bounded retry with the same IDs |
| Timeout, connection loss or invalid acknowledgement | Delivery uncertain; retain IDs and retry |
| Untrusted TLS certificate | Fix trust/hostname; never disable verification |

Application errors include `error`, `message`, `request_id` and `timestamp`;
`X-Request-ID` is also returned. The supplied helpers expose a validated UUID
request ID and status without copying response/error bodies into exceptions.
Their defaults are three attempts with 100/200 ms backoff and an eight-second
network deadline per attempt, shared by DNS, all address connections, TLS and
HTTP. Configure one to ten attempts. Python cannot cancel an operating-system
DNS lookup: at most eight daemon resolver workers may finish later. They never
connect or receive events or keys; waiting for a resolver slot uses the same
deadline. The helpers do not follow redirects or publish credentials to another
endpoint.

`GET /health` checks the API process. `GET /ready` checks API broker readiness;
it says nothing about database persistence. Public metrics and administration
routes return 404. Consumer health/metrics are internal. Use the SQL verification
command in the deployment guide or the [operator guide](operations.md) to check
stored IDs and consumer progress.
