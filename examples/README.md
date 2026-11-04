# Application examples

- [Checkout](checkout/README.md): copyable Python/Node HTTP clients, single/batch
  fixture CLIs, explicit CA trust and bounded stable-ID retries.
- [Transactional outbox](outbox/README.md): atomically store a business order and
  serialized event, then dispatch safely across process restarts.
- [Backend integration guide](../docs/integration.md): import and use the clients
  from application code.
- [HTTP contract](../docs/api.md): limits, responses, retries and persistence checks.

Start with the [self-hosted deployment](../deploy/README.md); the application
examples do not require Go or a package registry SDK.

A 202 acknowledges the broker. Verify storage separately when your workflow
requires it. Keep API keys in your backend and retain event IDs across restarts.
