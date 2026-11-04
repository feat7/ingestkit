# Run and read the experiments

The experiments answer three questions: how to write batches to PostgreSQL,
what happens at different HTTP loads, and whether saved events survive specific
outages. A case is one fixed configuration, measured three times after warm-up.
The registered study has 11 database cases, 11 HTTP cases and 3 recovery cases.

Start with the [current writer results](results/selfhost-2026-11-04/README.md).
The [earlier COPY-writer study](results/selfhost-copy-2026-11-04/README.md) is also
complete. Batch INSERT had a lower median write time than COPY/staging/merge in
all 11 database cases, so the deployed worker changed to batch INSERT and the
whole study ran again. These are local measurements with recorded limits.

## Run a small check before a full study

The experiment host needs Go 1.24+, Docker/Compose 2.24.4+, Python 3.9+ and OpenSSL.
The driver starts its own HTTPS deployment and trusts its extracted private CA.
It publishes observation ports only on loopback, saves evidence, then removes
only that run's services. A deployment server itself does not need Go installed.

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

The command prints its output directory and each case's status. With no path,
it chooses a fresh ignored `tmp/studies/<UUID>` directory. An existing directory
requires `-resume`. Full measurements need clean committed source. Smoke runs
may be explicitly marked dirty and test correctness; they are not publishable
timing evidence.

Run timing studies one at a time. Keep source, concurrent tests and figure jobs
unchanged during a comparison, and leave unrelated services alone. The recorded
host had other workloads; that shared use is part of its reported limitations.

## What changes between cases

Database methods rotate order across the three runs. These are matched sweeps,
not every possible combination of settings.

| Question | Configurations | Settings kept the same |
| --- | --- | --- |
| SQL batching | Batches 1/10/50/100/500/1000 | 5,000 submissions/method, 256 B, no duplicates |
| SQL payload size | 1 KiB/16 KiB | Batch 100; compare with 256 B/batch 100 above |
| Repeated IDs | 10/50/90% | Batch 100, 256 B; compare with 0% above |
| HTTP batching | Batches 1/10/100 | Target 1,000 events/s, 256 B, 32 clients |
| HTTP sending rate | 100/5,000/10,000 events/s | Batch 100, 256 B, 32 clients; 1,000/s baseline above |
| HTTP payload size | 1 KiB/16 KiB | Batch 10; compare with 256 B/batch 10 above |
| Burst | Same total sent in the first quarter of the window | Batch 100, 256 B, 32 clients |
| Concurrent clients | 8/64 clients | Batch 1, target 1,000/s, 256 B; 32-client baseline above |
| PostgreSQL outage | Stop requested at 3s; start at 8s | 15-second target of 500/s, batch 100 |
| Worker restart | KILL requested at 3s; start at 5s | Same 15-second target of 500/s |
| Outbox with broker outage | 100 saved application events; broker stopped for 8s after stop completion | Failed attempt lasts more than 5s; a new dispatcher sends the original UUIDs |

HTTP load cases run for ten seconds after two seconds of warm-up. SQL observation
aims for a query every 50 ms and allows 30 seconds to finish observing outstanding
events. Large payloads use batch 10 to fit the 1 MiB request limit. The study fixes
the API request limit at 10,000/s, info logging, broker allocation at one core/1 GiB,
and topic retention at seven days. Actual process flags, topic/database settings
and verified HTTPS protocol are captured. Environment variables cannot silently
retune a matched case.

Inputs are varied ASCII JSON at the exact requested byte size, generated
reproducibly from the recorded seed and namespace. The namespace separates one
run's event IDs from another's. Duplicate submissions keep the same IDs and bytes.

For the database comparison, input generation, table reset, warm-up and content
checks happen outside the write timer. Single INSERT commits every row; batch
INSERT and the independent COPY/staging/merge method commit every batch. They
use the same indexed table and keep the first stored content for a repeated ID.
Submitted events/s includes repeats; unique inserted rows/s counts only new rows.

## Read the counts before the rate

An offered rate is what the driver tried to schedule. Attempted events are what
it actually sent. Acknowledged events have a successful matching HTTP response.
Verified stored events are the unique rows found by the independent final check.
A sending schedule the client missed is not a record the collector lost.

| Measure | What is sampled or counted |
| --- | --- |
| HTTP ACK latency | One sample for each successfully acknowledged request |
| SQL visibility upper bound | One sample for each event first observed after commit, from API receipt through completed SQL observation; includes polling/query delay |
| In-window ACK rate | Responses finished within the target window, divided by that window |
| Verified stored rate | Unique verified rows divided by the window, unfinished requests, observation and final verification together |

`ObservedEvents` counts events with latency samples. `VerificationNS` records
final-check time. A row found only at the final check gets no invented latency.
The observer tracks unseen IDs in a temporary table on its own connection, so
out-of-order commits remain observable. The final check independently compares
IDs, types, times and payloads, and touches only that run's namespace/IDs. It
never truncates a shared table or cleans another run's rows.

A `limited` result keeps missed schedules, late starts, request/ACK tails,
rejections, uncertain delivery, delayed polls and observer outages visible.
A response just outside a finite window does not establish saturation. An
uncertain ID absent at the final deadline stays unknown; absence then does not
prove it can never arrive later. Missing acknowledged IDs, unexpected content/IDs
or duplicate physical rows fail correctness and cannot become a speed result.

p50/p95/p99 use the nearest-rank calculation on their separate raw samples.
For example, p95 describes the time by which 95% of those samples finished.
Receipt and SQL observation use the deployment clock; the host schedules load
with monotonic time. Reports include query/poll time and the largest observation
gaps. Only the deliberate PostgreSQL-outage experiment reconnects the observer
and rebuilds its unseen-ID list, while recording the SQL gap. It does not infer
commit times during the gap. Fault times record control requests/completions;
starting a container is not proof that processing has recovered. Recovery cases
are separate from capacity measurements.

## Find the files behind a result

| Artifact | Contents |
| --- | --- |
| `index.json`, `summary.csv` | All configurations, statuses, summaries, attempts and raw-file checksums |
| `manifest-N.json` | Source commit/fingerprint, host/VM, runtime settings and exact image IDs/digests |
| `raw/<case>-attempt-N.json.gz` | All methods/runs, raw timing, queue observations and ID sets |
| `resources-N.json.gz` | Coarse snapshots of this run's containers; these do not establish a bottleneck |
| `journal.json` | Progress, compatibility identity and the history of interrupted attempts |

Each file reference includes SHA256 and raw/compressed sizes. Gzip timestamps
are zero. A capture is limited to 128 MiB raw and 64 MiB compressed. HTTP ID sets
use compact submission indices with namespace/seed/count; outbox UUIDs are stored
explicitly. Private credentials, CA files and Docker/error logs remain in the
ignored run directory.

## Resume the same experiment

Resume keeps completed captures unchanged. A partially completed case runs all
three repetitions again on a fresh isolated stack and keeps its interrupted
history. It rejects a changed source/file set, seed, case controls, host/runtime
controls or image content/configuration. Exact image IDs remain in the manifests.
Compatibility ignores only Compose's project-specific image label, which changed
config IDs without changing filesystem or execution; other configuration/layers
remain part of the fingerprint. Source or configuration changes need a new study.

Ctrl-C/SIGTERM allows bounded cleanup of that run's resources. An uncatchable
process or host kill can leave them behind. Inspect the private evidence and
remove only that project; do not use global prune or benchmark over an abandoned
stack.

## Other commands and figures

`make bench-db` and `make bench-pipeline` run configurable single experiments
on development HTTP. `BENCH_FLAGS` controls count/batch/payload/duplicates or
rate/duration/concurrency/pattern; see `go run ./cmd/bench -h`. They still need
committed source and create their own stacks. They are separate from the registered
HTTPS study, which rejects ad-hoc workload flags.

The [figure guide](../docs/figures/README.md) explains the questions behind each
plot and how to regenerate it from the audited index. [Historical captures](results/local-2026-11-03/README.md)
retain a correction: topic write caching was off, but development mode still
bypassed process-level fsync. Their old raw bytes remain unchanged. The corrected
studies inspect and reject that bypass. No local capture here establishes maximum
capacity, cloud cost, production adoption or recovery from losing the only host/disk.
