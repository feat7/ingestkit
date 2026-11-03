# Application examples

- [Checkout](checkout/README.md): copyable Python/Node HTTP clients, single/batch
  fixture CLIs, explicit CA trust and bounded stable-ID retries.
- [Backend integration guide](../docs/integration.md): import and use the clients
  from application code.
- [HTTP contract](../docs/api.md): limits, responses, retries and persistence checks.

A 202 acknowledges the broker. Verify storage separately when your workflow
requires it. Keep API keys in your backend and retain event IDs across restarts.
