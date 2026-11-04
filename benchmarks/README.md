# Experiments

Read the [corrected self-hosted study](results/selfhost-2026-11-04/README.md) for all25 configurations, all repetitions and explicit limits, including ambiguous outcomes. The [earlier COPY-writer reference](results/selfhost-copy-2026-11-04/README.md) is preserved separately.

Run `make bench-db` with Docker running. Each invocation creates and destroys its own local stack, prints an evidence directory, and writes every measured repetition to JSON/CSV. No external database is accepted by the CLI.

Defaults: 10,000 submitted events, 256-byte JSON payload, 10% duplicate submissions, batch size 100, three repetitions after warm-up. Override via `BENCH_FLAGS='-count 1000 -batch 50 -duplicates 0'`. Single INSERT commits every row; batch INSERT and the explicit COPY/staging/merge reference commit every batch. All use the same table, indexes, conflict policy and database settings. These transaction boundaries are part of the experiment.

Workload generation, cleanup and exact ID/payload verification are outside the timer. Each repetition rotates method order. A random tenant scopes cleanup; no global truncation or schema changes. A missing row, mismatched payload, partial SQL failure or incomplete report fails the command. Raw per-call latency samples are in JSON; medians summarize repetitions. Historical captures are retained below; their broker durability correction is documented with the results.

Run `make bench-pipeline` for three measured windows after warm-up. `BENCH_FLAGS='-pattern burst -rate 1000 -duration 10s -batch 100 -concurrency 8'` offers the same total event count in the first quarter of each window. Steady load spreads it across the whole window. Rates are targets; inspect attempted events, queue misses, late schedules, HTTP failures and resource snapshots before interpreting a run.

HTTP acknowledgement latency is one sample per successful request. Storage latency is one sample per observed event: API receipt to first SQL observation after commit, including polling and query delay. Receipt and observation use the deployment VM clock: PostgreSQL clock_timestamp is queried after reading committed rows; host time schedules the load. The clock probe and observer transaction overhead are reported, not subtracted. ObserverLatePolls counts sampling-start gaps above twice the requested polling interval; ObserverMaxGapNS retains the largest gap. It is an upper bound, not a physical commit timer. Its population is the pending observer's ObservedEvents; rows first found by the final independent check do not acquire invented latency samples. VerificationNS records that final query's overhead, which remains in the total rate denominator. Persisted throughput divides independently verified unique events by the complete window plus request tail and observation/final-verification drain; acceptance throughput counts only acknowledgements completed inside the requested window. Acknowledgements after that window and requests that only start after it are separate counts. Raw samples and backlog observations are retained. Acknowledged missing IDs, unexpected IDs or duplicate physical rows fail reconciliation. Ambiguous HTTP delivery is reported separately; an ambiguous ID absent at the observation deadline remains unknown, not proven undelivered. Outcomes with generator misses, late schedules, rejections, ambiguous delivery or request/ACK tails are labelled limited rather than attained load or capacity. The driver does not retry requests.

The CLI requires committed source and records that revision, image content IDs/repository digests, database durability/index settings, complete topic configuration, host/VM versions and coarse Docker resource snapshots. It removes only its owned stack. Local credentials remain in a private ignored run directory; public reports contain a whitelisted configuration. Database resets use a random tenant; pipeline cleanup names only its generated IDs and event type and refuses pre-existing namespace/ID collisions. Each case uses a recorded namespace to isolate IDs and event type. A temporary pending-ID relation observes committed rows without a timestamp cursor; an independent final ID/content query checks the result. Varied payload profiles use reproducible ASCII entropy at the exact requested encoded length; fixed/empty profiles retain legacy repeated padding for older single-run comparisons. Both run on the same host as their generator and observer. These experiments do not estimate maximum capacity or cloud cost.

[Recorded local experiments](results/local-2026-11-03/README.md) include every
repetition, exact configuration and the observed COPY/staging disadvantage.
The [figure source guide](../docs/figures/README.md) explains generation from the
new study index, native diagrams and responsive desktop/mobile figures. The old
experiments.svg remains a historical asset, not current deployment evidence.


**Durability correction:** the historical pipeline captures used a broker with
process-level fsync bypass enabled by dev-container mode. They do not establish
performance for the corrected configuration. The current stack explicitly disables
that bypass, and new manifests record the effective running-process setting.
Rerun before making deployment-throughput claims; do not rewrite old raw samples.

## Registered HTTPS study

`make bench-study` runs 25 sequential configurations on an owned self-hosted
HTTPS deployment. It starts private PostgreSQL/Redpanda dependencies and trusts
the deployment's extracted local CA; it does not use a public ACME host or an
external database. No Go installation is needed on a deployment server, but the
local experiment driver requires Go, Docker Compose 2.24.4+, Python 3.9+ and
OpenSSL. Commit source changes before a full run.

~~~sh
# Small correctness run: one database and one HTTPS pipeline case, three reps.
make bench-study-smoke STUDY_FLAGS='-out tmp/studies/smoke-1 -seed 42'

# Full timing/availability study: eleven DB, eleven pipeline, three recovery cases.
make bench-study STUDY_FLAGS='-out benchmarks/results/my-study -seed 42'

# Resume an interrupted run using exactly its original source and controls.
make bench-study STUDY_FLAGS='-out benchmarks/results/my-study -seed 42 -resume'
~~~

The driver prints the chosen directory and each case's progress. A default
invocation chooses a new ignored `tmp/studies/<UUID>` directory. Existing output
is refused unless `-resume` is supplied. Study mode does not accept ad-hoc rate,
payload or batch flags: those would silently change the registered comparison.
Single-run database/pipeline commands remain available separately; their local
development route is not the HTTPS study route.

### Controls

- Database: 5,000 submissions, three methods with rotated order, three measured
  repetitions after warm-up. Batch sizes 1/10/50/100/500/1000; 1 KiB/16 KiB
  payloads at batch100; 10/50/90% duplicates at batch100 and 256B.
- Pipeline: ten-second windows, two-second warm-up, 30-second drain, 50 ms target
  SQL polling, 32 clients, 256B and 1,000 offered events/s by default. Sweeps
  cover batch1/10/100, rates100/5000/10000, payload1KiB/16KiB with batch10,
  a first-quarter burst, and 8/64 clients with batch1. Compare larger payloads
  with the 256B/batch10 control, not batch100.
- Recovery: DB down at3s/restoration requested at8s during a15s500/s run;
  consumer killed at3s/restarted at5s during the same workload; outbox100events
  with the broker stopped for eight seconds, exceeding a one-attempt publish
  deadline before replay. Each has three repetitions and recorded actual fault
  timestamps. Docker command duration and group recovery can shift availability;
  the figures must use actual observations, not the intended times.

The new registry uses deterministic varied ASCII payloads, not repeated padding.
Every case has its own namespace, recorded seed and exact byte controls. The
owned profile fixes API rate limit10,000, broker1core/1GiB and seven-day topic
retention independently of ambient shell variables; manifests verify actual
settings. The first complete study favored batch INSERT in all eleven DB configurations.
The deployed writer was changed separately with failure/replay tests; final
pipeline captures are recorded again from that committed code. The COPY reference
is independent of the current default and remains a distinct method.

During the deliberate database outage, the SQL observer explicitly records an
unavailable interval. It reconnects with remaining IDs after PostgreSQL returns;
it never advances a timestamp cursor or invents visibility samples in that gap.
Other pipeline SQL observation failures still invalidate the measurement. Final
independent ID/content reconciliation remains mandatory. Request samples retain
actual start/finish times and outcomes; backlog counts are reconstructed from
actual ACK times even when a SQL query blocks the driver.

### Artifacts and resume

- `index.json` and `summary.csv`: readable case controls, status, summary, attempt
  history and artifact checksums. `limited` is an observed driver/rejection/
  ambiguity/observer limitation, not an attained capacity claim.
- `manifest-N.json`: immutable, checksummed source/host/image/runtime provenance
  for each execution session. Full source SHA, actual source-file fingerprint,
  registry controls, image content/runtime config and host/runtime fingerprints
  must match before a resume can append measurements.
- `raw/<case>-attempt-N.json.gz`: every measured repetition/method, raw latency
  and request/backlog samples, and ID population evidence. Gzip timestamps are
  zero and compressed bytes are SHA-256 checked. Pipeline IDs use compact
  submission indices plus namespace/seed/count; outbox UUIDs are explicit.
- `resources-N.json.gz`: coarse owned-container snapshots, never an isolated
  bottleneck measurement. The generator and observer share the physical host.
- `journal.json`: atomic progress checkpoint. A partially completed case restarts
  all three repetitions on a fresh owned stack, retaining the interrupted capture;
  completed cases and their artifact bytes stay immutable. Source/config changes
  require a new output directory. Existing unrelated data is preserved.

Per-capture bounds are128MiB raw and64MiB compressed. Secret-bearing deployment
configuration, CA files and private Docker/error evidence remain in the ignored
owned run directory printed by the driver. They are not public result artifacts.
Manifests retain exact image IDs. Resume identity compares content layers and
runtime configuration while excluding only Compose's per-project image label,
which changes the image config ID without changing its filesystem or execution.

Smoke runs are correctness checks and may use an explicitly marked dirty source;
they are never publication timing evidence. Full studies require clean committed
inputs. Interrupt with Ctrl-C/SIGTERM for bounded owned cleanup. A hard process/
host kill can leave resources: inspect its private run evidence and clean only
that project, rather than using a global prune or measuring over an abandoned run.
Full timing studies stay out of ordinary CI; CI exercises the small study and
real recovery correctness cases.
