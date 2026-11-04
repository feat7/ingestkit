# Use IngestKit from a backend

Deploy the [self-contained server](../deploy/README.md), then send ordinary HTTPS
requests. The [HTTP contract](api.md) is the interface; you can copy either small
helper into your project without installing an SDK or running Go there. The
Python example requires Python 3.9+; the Node example requires Node 20+.

Keep API keys on a trusted backend. Browser/mobile clients should call your own
backend, which decides the event and supplies identity. Map overlapping keys to
the same tenant during [key rotation](operations.md). Do not send a tenant ID in
the event body.

## Python

From this checkout, the module can be imported directly. If copying the helper,
change the import to `from ingest_client import Client, IngestError`.

~~~python
import os
from datetime import datetime, timezone
from uuid import uuid4
from examples.checkout.ingest_client import Client, IngestError

client = Client(os.environ["API_URL"], os.environ["INGESTKIT_API_KEY"])
event = {
    "event_id": str(uuid4()),
    "event_type": "order_completed",
    "occurred_at": datetime.now(timezone.utc).isoformat(),
    "payload": {"order_id": "order-123", "amount_cents": 2500},
}
# Save this event with the business operation before attempting delivery.
try:
    acknowledged_ids = client.send(event)
except IngestError as error:
    # Inspect error.status, error.retryable, error.request_id.
    # Keep the saved event for a later retry or visible rejection workflow.
    raise
~~~

Use `client.send_batch(events)` for a list. Both methods return the validated
acknowledgement IDs. Construct the event once and retain it across retries.

## Node

Copy `examples/checkout/ingest-client.mjs` into your backend, or use this import
when running a module from the repository root:

~~~js
import {randomUUID} from 'node:crypto';
import {IngestClient, IngestError} from './examples/checkout/ingest-client.mjs';

const client = new IngestClient({
  url: process.env.API_URL,
  key: process.env.INGESTKIT_API_KEY,
});
const event = {
  event_id: randomUUID(),
  event_type: 'order_completed',
  occurred_at: new Date().toISOString(),
  payload: {order_id: 'order-123', amount_cents: 2500},
};
// Save event with the business operation before attempting delivery.
try {
  const acknowledgedIDs = await client.send(event);
} catch (error) {
  if (error instanceof IngestError) {
    // error.status, error.retryable and error.requestID are safe metadata.
    // Keep event for a later attempt or a visible rejection workflow.
  }
  throw error;
}
~~~

Use `client.sendBatch(events)` for a list. Both modules serialize once per call,
reuse identical bytes across bounded retries, validate acknowledgement IDs and
leave the original event object unchanged. Keep event IDs and bodies when a
process exits: an in-memory retry loop is not durable delivery.

## Try both callers

After loading `INGESTKIT_API_KEY` as shown in the deployment guide:

~~~sh
export API_URL=https://events.example.com
export INGESTKIT_API_KEY
python3 examples/checkout/send.py
SEND_MODE=single node examples/checkout/send.mjs
~~~

Both use the same fixed checkout UUIDs, so running them repeatedly still stores
three logical events for that tenant. Set `EVENTS_FILE` to your own JSON fixture
with an `events` array, and use fresh UUIDs for new business events. The CLIs fail
with a nonzero exit status when acknowledgement is absent or invalid.

Public HTTPS uses the system CA roots. For an owned local deployment with a
private CA, set `INGESTKIT_CA_FILE=/absolute/path/to/root.crt`; Python's `ca_file`
and Node's `caFile` options provide the same trust. No certificate bypass is used.

The [transactional outbox example](../examples/outbox/README.md) implements the
order→dispatch→SQL workflow with SQLite, including a real after-ACK crash test.

An HTTP send after committing your application's transaction still leaves a gap:
the process can die between those operations. An application outbox closes that
creation gap by saving the business row and serialized event in one local
transaction, then dispatching it with stable IDs. The collector's deduplication
handles dispatch retries; it does not make your application transaction and HTTP
request atomic.

## Choose a delivery boundary

Use the direct clients when the application already retains work durably and can
retry it, or when occasional loss before sending is acceptable for that event.
Use an application outbox when creating the business state must also create
pending delivery atomically. In both cases, keep UUIDs/content across uncertain
responses; a matching 202 is broker acknowledgement, not a database query result.

The [recovery captures](../benchmarks/results/selfhost-2026-11-04/README.md) include
three broker-outage/outbox repetitions with 100 saved events each. The separate
after-ACK child-kill regression checks the local marking gap exactly. The
[delivery guide](delivery.md) explains the independent consumer SQL/offset gap.
