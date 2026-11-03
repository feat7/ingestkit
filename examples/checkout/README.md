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
