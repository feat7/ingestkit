# HTTP API

Send JSON to the HTTPS URL from the [deployment guide](../deploy/README.md).
Every write needs `Authorization: Bearer <key>` and
`Content-Type: application/json`. The key selects the tenant, a name stored
with each event so the same UUID can exist independently for different tenants.
Callers do not send a tenant in the event body.

## Send one event or a batch

`POST /v1/events` accepts one event envelope, the fields around your payload:

~~~json
{
  "event_id": "dc12060f-f3a6-4ed3-9ea9-2d396b260b41",
  "event_type": "order_completed",
  "occurred_at": "2026-11-04T00:00:00Z",
  "payload": {"order_id": "order-123", "amount_cents": 2500}
}
~~~

`POST /v1/events/batch` accepts `{ "events": [ ... ] }`.
Save a UUID with the business operation and keep its content for retries.
Use a new UUID for a new event. If a send times out, retry the saved UUID and
content. The first stored `(tenant_id,event_id)` wins; sending new content for
that key does not update it.

The payload must be a JSON object. Event types are 1 to 128 bytes, and timestamps
must parse as RFC 3339. The API rejects malformed JSON, unknown envelope fields
and values PostgreSQL JSONB cannot store. [Delivery details](delivery.md) explain
the Unicode, numeric and nesting restrictions.

| Limit | Value |
| --- | --- |
| Encoded individual input event | 256 KiB |
| HTTP request body | 1 MiB |
| Events in a batch | 1 to 100 |
| Payload nesting | At most 64 levels |

For large events, reduce the batch size to fit the whole request. Ten 16 KiB
payloads fit the request limit; one hundred do not.

The deployment's request limiter adds 10,000 request tokens per second per tenant
and permits a burst of 10,000 requests. A batch consumes one request token.
Those settings limit requests; they do not establish measured event capacity.

## Read the response before marking an event sent

A successful response is HTTP 202:

~~~json
{"status":"accepted","event_ids":["dc12060f-f3a6-4ed3-9ea9-2d396b260b41"],"request_id":"f8c22d85-5e93-4cb5-a456-18b0f104bcca"}
~~~

The IDs must match the input order. This confirms that the queue acknowledged
the events. It does not wait for the database worker or PostgreSQL. An HTTP batch
is not one transaction across all services. A failed or timed-out request can
have delivered some or all records; retry the original IDs and content.

| HTTP result | What to do |
| --- | --- |
| 202 with matching IDs | The queue accepted the events; check persistence separately when needed |
| 400 | Fix the invalid event instead of retrying it unchanged |
| 401 | Fix or rotate the key; keep unsent business events |
| 404 or redirect | Correct the URL or endpoint settings |
| 413 | Reduce event/request size or split the batch |
| 429 | Retry the same content after a bounded delay |
| 500/502/503 or another server failure | Keep the event; retry its original ID because delivery may be uncertain |
| Timeout, connection loss or invalid response IDs | Keep the event and retry the same ID |
| Untrusted TLS certificate | Fix the hostname or trusted CA; keep certificate verification enabled |

Application errors have `error`, `message`, `request_id` and `timestamp` fields.
The response also carries `X-Request-ID`. The example helpers expose a validated
UUID request ID and status without putting response bodies in their exceptions.

They default to three attempts with 100/200 ms delays and an eight-second network
deadline per attempt. That deadline includes DNS, all address connections, TLS
and HTTP. You can configure one to ten attempts. Python cannot cancel an OS DNS
lookup, so at most eight daemon resolver workers may finish later. They never
connect or receive events or keys. Waiting for a resolver slot uses the same
deadline. Neither helper follows redirects or sends credentials to another URL.

## Check health and storage separately

`GET /health` checks the API process. `GET /ready` checks whether the API can
accept events through the broker. It cannot confirm a PostgreSQL write. Consumer
health and metrics stay on the private network; public metrics/admin routes
return 404. Use the deployment guide's SQL query or the [operator guide](operations.md)
to check stored IDs and worker progress.
