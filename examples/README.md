# Try the application examples

Start with the [self-hosted deployment](../deploy/README.md), then choose an
example. Neither requires Go in the application or a package-registry SDK.

- [Checkout](checkout/README.md) sends three events using the Python or Node
  client. Run it twice to see the same IDs produce one set of rows.
- [Order and outbox](outbox/README.md) saves an order and pending event together,
  then sends pending work from a separate process. Restart it to retry saved IDs.

The [integration guide](../docs/integration.md) shows the imports to use in your
own backend. The [API reference](../docs/api.md) lists fields, limits and retries.

Keep the key on your backend and keep event IDs across restarts. HTTP 202 confirms
queue acceptance; use SQL to confirm storage when your workflow needs it.
