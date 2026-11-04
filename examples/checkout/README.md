# Send three checkout events

The fixture has a product view, checkout start and completed order. Each has a
fixed UUID, so you can resend it and see PostgreSQL keep one row per event.

Follow the [server setup](../../deploy/README.md), then run from that checkout
with its initial generated key mapping. The commands reload the key after the
deployment demo clears it. On another backend host or after rotating the key,
supply API_URL and INGESTKIT_API_KEY through private application settings.
Use your collector hostname in API_URL.

~~~sh
export API_URL=https://events.example.com
INGESTKIT_API_KEY=$(sed -n 's/^API_KEYS={"\([0-9a-f]*\)":"default"}/\1/p' deploy/.env)
export INGESTKIT_API_KEY
python3 examples/checkout/send.py
SEND_MODE=single node examples/checkout/send.mjs
~~~

Both commands read `events.json` and keep those IDs on retry. Running them again
still leaves three logical events for that tenant. Give new business events
fresh UUIDs. The clients use the batch endpoint by default; `SEND_MODE=single`
sends individual requests. `EVENTS_FILE=/path/to/events.json` selects another
`{ "events": [...] }` fixture. HTTP 202 confirms that the queue accepted the
records; the worker writes PostgreSQL separately.

## Run a disposable local demo

With Docker and Go installed:

~~~sh
make demo DEMO_FLAGS=-cleanup
make demo-recovery DEMO_FLAGS=-cleanup
~~~

Each demo creates its own services and checks that accepted IDs reached storage.
Without `-cleanup`, it stops those services and keeps their volumes, then prints
the command for removing that run. The root `make up` development stack also
works with the callers' local defaults, including `local-demo-key`. Use the generated
private key for the deployed server.

## Use the same helper in your backend

`ingest_client.py` exports `Client.send`, `Client.send_batch` and `IngestError`.
`ingest-client.mjs` exports `IngestClient.send`, `sendBatch` and `IngestError`.
Importing either module does no I/O, and neither needs third-party packages.
The [integration guide](../../docs/integration.md) shows imports and calls; the
[API reference](../../docs/api.md) explains response checking and retry decisions.

Set API_URL and INGESTKIT_API_KEY for your HTTPS deployment. Public certificates
use system trust. For local tests, `INGESTKIT_CA_FILE` names a private CA to trust.
Both helpers leave certificate checks enabled.

Run the client tests, including TLS and interrupted requests:

~~~sh
python3 -m unittest discover -s examples/checkout -p 'test_*.py'
node --test examples/checkout/*.test.mjs
~~~
