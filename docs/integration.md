# Send events from your application

Deploy the [collector](../deploy/README.md), then call it over HTTPS from your
backend. The [API reference](api.md) describes the request and response. You can
copy either helper into your project; they need no SDK installation or Go runtime.
Use Python 3.9+ or Node 20+.

Before choosing a client, decide where an unsent event will live. If your
application already saves work and can retry it after a restart, the direct
clients below can send that work. For an event that must be saved with a business
change, use the [order/outbox example](../examples/outbox/README.md). It commits
the order and its pending event together, then sends outside that transaction.
A process can die between a standalone order commit and an HTTP call; retries
alone cannot recover an event that was never saved.

Keep keys on your backend. A browser or mobile app calls your backend, which
chooses the event and its identity. The key selects the tenant; leave tenant ID
out of the event body. During [key rotation](operations.md), both keys can point
to that same tenant.

## Try the included events

From the collector checkout with its initial generated key mapping, run the
following commands. They load the key again after the deployment demo clears it.
Use your collector hostname in API_URL.
For another backend host or a rotated key, supply `API_URL` and
`INGESTKIT_API_KEY` through that application's private settings.

~~~sh
export API_URL=https://events.example.com
INGESTKIT_API_KEY=$(sed -n 's/^API_KEYS={"\([0-9a-f]*\)":"default"}/\1/p' deploy/.env)
export INGESTKIT_API_KEY
python3 examples/checkout/send.py
SEND_MODE=single node examples/checkout/send.mjs
~~~

The two commands use the same three checkout UUIDs. Running either repeatedly
still leaves three logical events for that tenant. `EVENTS_FILE` selects a JSON
fixture with an `events` array; give new business events fresh UUIDs. The commands
exit nonzero if the response is missing or its IDs do not match.

HTTP 202 means Redpanda accepted those IDs. It does not tell you whether the
worker has finished the PostgreSQL write. Check SQL when your workflow needs
that confirmation, and keep IDs/content after an uncertain response.

## Python

This import works from the repository. If you copy the helper into your project,
use `from ingest_client import Client, IngestError` instead.

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

Create the event once, save it and reuse it on retry. Calling `client.send_batch(events)`
sends a list; both methods return only the IDs whose response they have checked.
Save the UUID along with the event body so a replacement process has the same work.

## Node

Copy `examples/checkout/ingest-client.mjs` into your backend, or use the import
below from a module in the repository root:

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

Use `client.sendBatch(events)` for a list. Both helpers serialize once per call,
keep the original object unchanged, and resend identical bytes during their
bounded retries. The application's saved event must also survive process exit;
the client cannot recover an object that existed only in memory.

## HTTPS trust and recovery

Public HTTPS uses the system's trusted certificate authorities. For your local
test deployment, use `INGESTKIT_CA_FILE=/absolute/path/to/root.crt`, Python's
`ca_file` option or Node's `caFile` option to trust its private CA. The helpers
keep certificate verification enabled.

The outbox example includes a crash test after HTTP acknowledgement but before
the local pending row is updated. The [delivery guide](delivery.md) explains that
retry gap and the worker's separate SQL/offset gap. Retrying uncertain HTTP
outcomes must preserve the event IDs. Previous measured recovery bundles have
been archived outside the checkout. The [current connection study](../benchmarks/README.md)
contains short local screens, not a sustained deployment capacity guarantee.
