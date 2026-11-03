# Checkout with a transactional outbox

This is one small SQLite application, not another collector service. Creating an
order saves its business row and serialized event in the same transaction. A
separate, bounded dispatch command sends pending events through the reusable
Python HTTP client. The collector still acknowledges the broker before storage.

From the repository root, with the HTTPS collector deployed and its backend key
loaded as described in the [deployment guide](../../deploy/README.md):

~~~sh
umask 077
export API_URL=https://events.example.com
export INGESTKIT_API_KEY
python3 -m examples.outbox create-order --database private/orders.db \
  --order-id order-123 --amount-cents 2500
python3 -m examples.outbox status --database private/orders.db
python3 -m examples.outbox dispatch --database private/orders.db
python3 -m examples.outbox status --database private/orders.db
~~~

The first command prints its persisted event UUID. Repeating the same order ID
and amount returns that UUID; a conflicting amount fails. The UUID, occurred_at
and serialized payload do not change when the process restarts. Use a new order
ID for a new order. The private directory and SQLite business data belong on a
persistent disk and in your own application backup strategy.

`dispatch` attempts at most 100 pending records once, with bounded retries inside
the HTTP helper. `--batch-size 10` lowers the selection size; `--max-attempts 1`
limits transport attempts. Set `INGESTKIT_CA_FILE` when using an explicit local
CA. Public HTTPS uses ordinary certificate verification.

The command prints attempted/acknowledged/rejected counts and the remaining
pending count. It exits nonzero when the selected work could not be acknowledged
or was rejected. An empty queue exits successfully. States mean:

| State | Meaning |
| --- | --- |
| pending | Still needs broker acknowledgement; network ambiguity, authentication/configuration failures and server errors remain here |
| acknowledged | HTTP 202 with exactly the selected IDs; PostgreSQL may still be catching up |
| rejected | A 400/413 requires an operator decision; the event and safe error code remain inspectable |

`status` reports all state counts and a bounded list of problem IDs, attempts and
safe error codes. Attempts count dispatcher calls, not each internal HTTP retry.
No payload, key, response body or database URL is copied into error summaries.
Do not silently discard a rejected event. Decide whether to repair the business
input, split an oversized batch, retry, or compensate. This example deliberately
has no automatic rejection reset that could rewrite an already delivered event.

## The crash boundary

The dispatcher reads pending work, releases the SQLite connection, sends HTTP,
and only then marks the selected rows acknowledged in another transaction. It
holds no business write lock while waiting on the network. If it dies after the
broker ACK but before that update, the rows remain pending. The next process
resends their original IDs and content; PostgreSQL keeps one row per tenant/ID.
The integration test pauses a test-only wrapper precisely after a real ACK,
kills that child process, dispatches again and checks one stored row.

Run one dispatcher at a time. Adapting this to multiple dispatchers needs a lease
or claim protocol in your application's database; this example does not claim
that behavior. A timer/cron/application worker can invoke the one-shot command,
but deploy that scheduling only after choosing your application's lifecycle.

## Verify at the collector

Copy the created event UUID into this query on the collector host:

~~~sh
docker compose --env-file deploy/.env -f deploy/compose.yml exec -T postgres \
  psql -U ingestkit_admin -d ingestkit -c \
  "SELECT event_id,event_type,payload FROM ingestkit.events WHERE tenant_id='default' AND event_id='YOUR-EVENT-UUID';"
~~~

The payload includes the order ID and amount. Acknowledged outbox state alone
does not replace this persistence check.

To adapt the pattern, place your real business write and the serialized outbox
row inside the same database transaction. Keep the event ID stable, dispatch
outside the transaction, and mark acknowledgement only after validating the
response. An HTTP call after a standalone business commit leaves a creation gap
that retries alone cannot repair. SQLite here is a concrete teaching example;
you do not need another SQLite database when your application already uses
PostgreSQL or another transactional store.

~~~sh
python3 -m unittest discover -s examples/outbox -p 'test_*.py'
# Real outage and after-ACK process-kill checks over HTTPS:
scripts/test-integration.sh -run TestOutbox -v
~~~
