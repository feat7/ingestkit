# Save an order and its event together

This example is a small SQLite application. One transaction saves an order and
a pending event containing its UUID and exact JSON body. A separate dispatch
command sends pending events to IngestKit. If either process stops, the saved
event is still available for a later attempt.

Follow the [deployment guide](../../deploy/README.md), then run from that checkout
with its initial generated key mapping. These commands reload the key. On another
backend host or after key rotation, supply API_URL and INGESTKIT_API_KEY through
private application settings. Use your collector hostname in API_URL:

~~~sh
umask 077
export API_URL=https://events.example.com
INGESTKIT_API_KEY=$(sed -n 's/^API_KEYS={"\([0-9a-f]*\)":"default"}/\1/p' deploy/.env)
export INGESTKIT_API_KEY
python3 -m examples.outbox create-order --database private/orders.db \
  --order-id order-123 --amount-cents 2500
python3 -m examples.outbox status --database private/orders.db
python3 -m examples.outbox dispatch --database private/orders.db
python3 -m examples.outbox status --database private/orders.db
~~~

The create-order command prints the event UUID. Creating that order again with
the same amount returns the same UUID; a conflicting amount fails. Restarting
the process leaves the UUID, occurred_at and serialized payload unchanged. Use
a new order ID for a new order. Keep the private directory on persistent disk
and include this SQLite business data in your application's backup plan.

## Read the dispatch result

`dispatch` selects at most 100 pending records per invocation. The HTTP helper
makes bounded retries for that work. `--batch-size 10` lowers the selection size;
`--max-attempts 1` allows one transport attempt. Set `INGESTKIT_CA_FILE` for an
explicit local CA. Public HTTPS uses ordinary certificate verification.

The command prints attempted, acknowledged and rejected counts, plus the number
still pending. It exits nonzero if selected work was rejected or could not be
acknowledged. An empty queue succeeds.

| State | What it means |
| --- | --- |
| pending | The event still needs queue acknowledgement. Network uncertainty, authentication/configuration failures and server errors leave it here. |
| acknowledged | HTTP 202 named exactly the selected IDs. PostgreSQL may still be catching up. |
| rejected | A 400/413 needs an operator decision. The saved event and safe error code remain available. |

`status` prints counts and a bounded list of problem IDs, attempts and safe error
codes. An attempt counts a dispatcher call, not every internal HTTP retry.
Error summaries exclude payloads, keys, response bodies and database URLs.

Inspect rejected events and decide whether to fix input, split a batch, retry
or compensate for a business operation. The example has no automatic rejection
reset, since rewriting an event that might already have arrived needs a decision.

## If dispatch stops after HTTP 202

The dispatcher reads pending work, closes the SQLite connection and sends HTTP.
Only after checking the response IDs does it mark those rows acknowledged in a
new transaction. It holds no business write lock during the network wait.

If the process stops after the response but before that update, the row stays
pending. A new process sends the same UUID and content again. PostgreSQL's unique
tenant/event key keeps one row. The integration test pauses a separate test-only
wrapper after an actual acknowledgement, kills it, dispatches again and checks
that single row.

Run one dispatcher at a time. Multiple dispatchers need a claim or lease in your
application database. A timer, cron job or application worker can invoke this
one-shot command once you have chosen how it fits your application's lifecycle.

## Check the row at the collector

Copy the created UUID into this query on the collector host:

~~~sh
docker compose --env-file deploy/.env -f deploy/compose.yml exec -T postgres \
  psql -U ingestkit_admin -d ingestkit -c \
  "SELECT event_id,event_type,payload FROM ingestkit.events WHERE tenant_id='default' AND event_id='YOUR-EVENT-UUID';"
~~~

Check the order ID and amount in the payload. An acknowledged outbox row confirms
queue acceptance; it does not replace this storage check.

## Adapt it to your database

Put the business write and serialized outbox row in the same database transaction.
Keep the ID stable, send outside that transaction and mark acknowledgement after
validating the response. A separate business commit followed by HTTP still leaves
a gap where the process can die before creating any pending delivery.

Use your application's existing transactional database when you have one. SQLite
is here to make the example runnable; it is not a requirement for other projects.

~~~sh
python3 -m unittest discover -s examples/outbox -p 'test_*.py'
# Real outage and after-ACK process-kill checks over HTTPS:
scripts/test-integration.sh -run TestOutbox -v
~~~

The [owned HTTPS tests](../../tests/integration/outbox_test.go) cover a broker
outage longer than the publish deadline and a dispatcher killed after the ACK.
A new dispatcher resends pending events with their original IDs. These checks
supplement the transaction/worker checks needed for your own application. The
old recorded recovery bundle was archived; the current connection study does
not add a new measured outbox-recovery result.
