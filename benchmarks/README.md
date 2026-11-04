# Experiments

Read the [corrected self-hosted study](results/selfhost-2026-11-04/README.md) for
all 25 configurations, repetitions, controls and limits. The complete
[COPY-writer reference](results/selfhost-copy-2026-11-04/README.md) is preserved
separately. The final deployed writer uses batch INSERT, chosen after the first
study favored it over COPY/staging/merge in all 11 database configurations.

## Run the registered study

Use Go 1.24+, Docker/Compose 2.24.4+, Python 3.9+ and OpenSSL on the local experiment
host. The driver starts one owned HTTPS deployment, trusts its extracted private
CA, exposes only owned loopback observation ports, saves evidence and removes
only its own resources. The deployment server itself does not need Go.

~~~sh
# Small correctness subset; three repetitions per case, not publication evidence.
make bench-study-smoke STUDY_FLAGS='-out tmp/studies/my-smoke -seed 42'

# Full measurements: commit sources, run sequentially, choose a fresh directory.
make bench-study STUDY_FLAGS='-out benchmarks/results/my-study -seed 42'

# Resume using the exact original source, seed, controls and runtime identity.
make bench-study STUDY_FLAGS='-out benchmarks/results/my-study -seed 42 -resume'

# Independent read-only audit of a complete recorded study.
python3 benchmarks/audit_study.py benchmarks/results/selfhost-2026-11-04
~~~

The command prints the chosen output path, each case and its status. An omitted
path selects a fresh ignored `tmp/studies/<UUID>` directory. Existing output is
refused without `-resume`. Full runs require clean committed source; smoke may
be explicitly marked dirty and is never publication timing evidence. Do not
edit source, run concurrent test/plot workloads, or stop unrelated user services
while timing a comparison. The recorded host had existing workloads, noted in
its environment limitations.

## Fixed controls

Every configuration has three measured repetitions after warm-up. The database
methods rotate order. The registry is deliberately not a full Cartesian product.

| Question | Configurations | Matched controls |
| --- | --- | --- |
| SQL batching | Batches 1/10/50/100/500/1000 | 5,000 submissions/method, 256 B, no duplicates |
| SQL payload | 1 KiB/16 KiB | Batch 100; compare with 256 B/batch 100 above |
| Repeated IDs | 10/50/90% | Batch 100,256 B; compare with 0% above |
| HTTP batching | Batches 1/10/100 | Offered 1,000 events/s, 256 B, 32 clients |
| Offered load | 100/5,000/10,000 events/s | Batch 100,256 B, 32 clients;1,000/s baseline above |
| HTTP payload | 1 KiB/16 KiB | Batch 10; compare with 256 B/batch 10 above |
| Burst | Same total in first quarter of window | Batch 100,256 B, 32 clients |
| Client concurrency | 8/64 clients | Batch 1, 1,000 offered/s, 256 B;32-client baseline above |
| PostgreSQL outage | Stop requested 3s; start requested 8s | 15-second 500/s workload, batch 100 |
| Consumer kill/restart | KILL requested 3s; start requested 5s | Same 15-second 500/s workload |
| Outbox + broker outage | 100 durable application events, broker stopped 8s after stop completion | Failed attempt exceeds 5s; new dispatcher replays original UUIDs |

Ordinary pipeline windows are ten seconds after two-second warm-up, with a
30-second observation drain deadline and 50 ms target polling. Large payloads use
batch 10 to fit the existing 1 MiB request cap. The selfhost study fixes API limit
10,000 requests/s, info logging, broker 1 core/1 GiB and seven-day topic retention.
Actual process arguments, topic/database settings and trusted HTTPS protocol are
captured; no environment variable silently tunes matched cases.

Payloads are reproducible varied ASCII JSON at the exact encoded byte length.
Each case/repetition uses a recorded namespace and seed. Duplicate submissions
reuse identical bytes/IDs. For SQL, generation/reset/warm-up/content verification
are outside elapsed time. Single INSERT commits each row; batch INSERT and the
independent COPY/staging/merge reference commit each batch against the same
indexed table/conflict policy. Submitted/s and inserted unique/s differ when
there are duplicates; those cases do less new-row work.

## Interpret the populations

- HTTP ACK latency: one sample per successfully acknowledged request.
- SQL visibility upper bound: one sample per event first seen by the pending-ID
  observer after commit, from API receipt through completed SQL observation.
  It includes polling/query delay, not just database execution.
- In-window ACK rate: only acknowledgements completed inside the target window,
  divided by that window. Late starts/ACKs are separate counters.
- Verified stored rate: independent unique-row count divided by the full window,
  request tail, observation drain and final verification. It is not an intrinsic
  server-capacity estimate.
- `ObservedEvents` identifies latency samples. `VerificationNS` records the final
  census overhead; rows found only then do not receive invented latency samples.

The temporary pending-ID relation is connection-owned, not a timestamp cursor.
Transactions committing out of order remain observable. Final ID/type/time/payload
reconciliation is independent of observation bookkeeping and scopes only the
owned namespace/IDs. It never truncates a shared table or cleans an unrelated run.

`limited` retains generator misses, late schedules, request/ACK tails, rejected or
ambiguous delivery, late polls and observer outages. A normal finite-window tail
does not by itself diagnose saturation. Fewer attempted events means the driver
missed its target; it does not mean the collector lost unattempted events.
An ambiguous ID absent at the final deadline remains **unknown**, not proven
undelivered. Acknowledged missing IDs, unexpected content/IDs or duplicate physical
rows fail correctness; they cannot become a successful speed result.

Receipt and SQL observation share the deployment clock; host monotonic time
schedules load. Nearest-rank p50/p95/p99 are computed from their distinct raw
populations. Slow polls/query duration/max gaps are reported. During the deliberate
PostgreSQL outage only, the observer reconnects and rebuilds pending IDs while
recording unavailable observations. No exact commit time is inferred in that gap.
Fault timestamps record actual control requests/completions, not an assumed
instant service recovery. Availability experiments are separate from capacity.

## Artifacts and resume

| Artifact | Contents |
| --- | --- |
| `index.json`, `summary.csv` | Every configuration, status, summaries, attempts, raw references and checksums |
| `manifest-N.json` | Immutable source SHA/fingerprint, host/VM, runtime settings, exact image IDs/digests |
| `raw/<case>-attempt-N.json.gz` | Every repetition/method, raw samples/backlog and ID-population evidence |
| `resources-N.json.gz` | Coarse owned-container snapshots; not proof of a bottleneck |
| `journal.json` | Atomic progress, compatibility identity and retained interrupted history |

Gzip timestamps are zero; references include SHA256 and raw/compressed sizes.
Per-capture limits are 128 MiB raw/64 MiB compressed. Pipeline populations use compact
submission indices plus namespace/seed/count; outbox UUIDs are explicit. Credentials,
CA files and private Docker/error evidence remain in ignored owned run directories.

A partial case reruns all three repetitions on a fresh owned stack, retaining
history and completed artifact bytes. Resume rejects changed source/files, seed,
registry controls, runtime/host controls or image content/configuration. Exact image
IDs remain in manifests; compatibility excludes only Compose's per-project image
label, proven to alter config IDs without changing filesystem/execution. Other
config and layers remain fingerprinted. Source/config changes need a new study.

Interrupt with Ctrl-C/SIGTERM for bounded owned cleanup. A hard host/process kill
can leave resources: inspect that run's private evidence and clean only that
project. Never use global prune or measure over an abandoned owned stack.

## Other entry points

`make bench-db` and `make bench-pipeline` retain configurable single-run experiments
on development HTTP. Their `BENCH_FLAGS` include count/batch/payload/duplicates or
rate/duration/concurrency/pattern as appropriate; inspect `go run ./cmd/bench -h`.
They are not the registered HTTPS deployment comparison. All single-run measurements
still require committed source and own their stacks. Registered study mode rejects
ad-hoc workload flags rather than silently changing the comparison.

See [figure sources](../docs/figures/README.md) for charts generated from the audited
index. [Historical local captures](results/local-2026-11-03/README.md) retain their
fsync-bypass erratum: topic write caching was disabled while development mode
still bypassed process-level fsync. They do not establish corrected deployment
throughput. The new studies inspect and reject that bypass; old raw bytes remain
unchanged. None of these local captures establishes maximum capacity, cloud cost,
production adoption or tolerance of the only disk/host failing.
