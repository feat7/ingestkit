# Checkout events

Three ordinary envelopes describe a product view, checkout start and completed
order. The fixed UUIDs make retry/deduplication easy to observe.

For a deployed collector, follow the [server setup](../../deploy/README.md), load
its backend key and set the URL:

~~~sh
export API_URL=https://events.example.com
export INGESTKIT_API_KEY
python3 examples/checkout/send.py
SEND_MODE=single node examples/checkout/send.mjs
~~~

These read `events.json` and retain its IDs on retry. Running both repeatedly
still stores three logical events for that tenant. Use fresh explicit UUIDs for
new business events. Both default to the batch endpoint; `SEND_MODE=single` sends
individual requests and `EVENTS_FILE=/path/to/events.json` selects your own
`{ "events": [...] }` fixture. A 202 acknowledges the broker, not PostgreSQL.

For an isolated local walkthrough with Docker and Go:

~~~sh
make demo DEMO_FLAGS=-cleanup
make demo-recovery DEMO_FLAGS=-cleanup
~~~

Each owns its services and checks accepted/stored IDs. Omitting `-cleanup` stops
its services and preserves volumes; the command prints its exact cleanup recipe.
The root `make up` development stack also supports the callers' local defaults,
including `local-demo-key`; those defaults are not deployed credentials.

## Copy the client into an application

`ingest_client.py` exports `Client.send` / `Client.send_batch` and `IngestError`.
`ingest-client.mjs` exports `IngestClient.send` / `sendBatch` and `IngestError`.
The modules perform no work at import time and have no package dependencies.
See the [backend guide](../../docs/integration.md) for imports and usage, and the
[API contract](../../docs/api.md) for acknowledgement and retry rules.

For the self-hosted HTTPS deployment, set API_URL and INGESTKIT_API_KEY. Public
certificates use normal system trust. `INGESTKIT_CA_FILE` selects an explicit
private CA for local tests; neither helper disables certificate checks.

Run their controlled-server tests (including TLS and interrupted requests):

~~~sh
python3 -m unittest discover -s examples/checkout -p 'test_*.py'
node --test examples/checkout/*.test.mjs
~~~
