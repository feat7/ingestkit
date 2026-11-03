# COPY-writer reference study

Completed 4 November 2026 from clean source `dccf4b3`. This is the first corrected
self-hosted HTTPS study, before the evidence-driven switch of the deployed writer
to batch INSERT. Keep these captures as a reference; the final writer study is
[recorded separately](../selfhost-2026-11-04/README.md).

All 25 registered cases completed: 99 database method/repetition rows, 33 pipeline
windows and 9 recovery repetitions. Nine cases are explicitly limited. Independent
Python checks reconciled raw samples, nearest-rank percentiles, request/ID
populations, rates, summaries, immutable checksums, registered controls and runtime
durability/HTTPS settings. An exact scan against this owned run’s generated
credentials passed. Each case uses three measured repetitions; all accepted event
IDs reconciled with independently verified content. Limits are retained in index
and raw captures and do not establish maximum capacity.

Batch INSERT had the lower median elapsed time than COPY/staging/merge in all
11 database configurations. This motivates the separate writer change and new
measurements; it does not make a universal claim about PostgreSQL COPY.

Source and actual image/runtime/host controls are in [manifest-1.json](manifest-1.json).
The generator and SQL observer shared an M1 Max physical host with the Docker VM
and existing workloads. Only owned containers were controlled. PostgreSQL fsync
and synchronous commit were on; the broker’s process bypass was false and topic
write caching was false. One broker/replica is not protection from host/disk loss.
Local private-CA HTTPS was verified; public certificate issuance was not tested.

Read [index.json](index.json) for all case controls, summaries, limits and raw
artifact references, [summary.csv](summary.csv) for the case listing, and
[journal.json](journal.json) for progress history. Run `python3
benchmarks/audit_study.py benchmarks/results/selfhost-copy-2026-11-04` from the
repository root for a read-only independent audit. Raw gzip files have zero
timestamps and SHA-256 references; no failed/limited cases were discarded.
