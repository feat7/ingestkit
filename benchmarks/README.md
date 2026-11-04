# Measurement revalidation

Previous public result bundles and measured charts have been withdrawn. An audit
found that synchronous SQL observation could delay HTTP-result bookkeeping after
stored rows were already visible. Some local screens were also constrained by
the load driver's worker count. Their rates cannot establish a server ceiling.

The retired bytes are preserved outside the checkout with source revisions and
checksums. A replacement study is in progress; no current repository result
supports a maximum or sustained-capacity headline.

## What a result must distinguish

- Offered events, scheduled jobs, started requests, acknowledged events and
  independently verified stored IDs/payloads.
- HTTP request concurrency, active TCP limits, retained idle connections,
  producer record/byte buffers and database pools.
- Observation health, delivery verification and load completeness. Successful
  delivery can coexist with a failed visibility measurement or unsent load.
- Actual commit/offset progress and polling delay. Final verification and
  bookkeeping time are not a direct SQL write-capacity measurement.

Measurements retain TLS verification, broker fsync, topic write caching off,
PostgreSQL durability, duplicate protection and SQL-before-offset progress.
The test stack is owned and isolated; cleanup never uses global prune.

## Check the harness

Run unit checks from the repository root:

~~~sh
go test ./internal/benchmark
python3 -m unittest discover -s benchmarks -p 'test_audit_study.py'
~~~

The supported deployment and clients remain documented in the
[deployment guide](../deploy/README.md) and [integration guide](../docs/integration.md).
The [figure guide](../docs/figures/README.md) retains source-backed architecture
and event-delivery diagrams. Connection-study commands and audited results will
be added after the measurement repair is verified.
