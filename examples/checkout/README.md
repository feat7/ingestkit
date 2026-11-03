# Checkout events

Three ordinary envelopes describe a product view, checkout start and completed
order. The fixed UUIDs make retry/deduplication easy to observe.

~~~sh
make demo
make demo-recovery
~~~

Each command starts an isolated stack, prints acknowledgements and stored IDs,
reconciles them, saves evidence, and stops with volumes preserved.
Use DEMO_FLAGS=-cleanup to remove only the run's resources after collecting evidence.

With make up running, use either caller without installing dependencies:

~~~sh
python3 examples/checkout/send.py
node examples/checkout/send.mjs
~~~

Both read events.json and retain its IDs on retry. Running both against the same
stack still stores three logical events. Supply API_URL and INGESTKIT_API_KEY
to use the same contract from another application. The included key is local only.

Both callers default to the batch endpoint. Use `SEND_MODE=single` to send the
same fixture as individual requests, or `EVENTS_FILE=/path/to/events.json` for
your own `{ "events": [...] }` fixture with stable explicit IDs. For example:

~~~sh
SEND_MODE=single python3 examples/checkout/send.py
SEND_MODE=single node examples/checkout/send.mjs
~~~

Each event or batch keeps identical serialized bytes across its bounded retries.
A 202 acknowledges broker delivery; verify storage separately.

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
