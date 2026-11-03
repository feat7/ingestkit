# Experiments

Run `make bench-db` with Docker running. Each invocation creates and destroys its own local stack, prints an evidence directory, and writes every measured repetition to JSON/CSV. No external database is accepted by the CLI.

Defaults: 10,000 submitted events, 256-byte JSON payload, 10% duplicate submissions, batch size 100, three repetitions after warm-up. Override via `BENCH_FLAGS='-count 1000 -batch 50 -duplicates 0'`. Single INSERT commits every row; batch INSERT and the production COPY/staging/merge store commit every batch. All use the same table, indexes, conflict policy and database settings. These transaction boundaries are part of the experiment.

Workload generation, cleanup and exact ID/payload verification are outside the timer. Each repetition rotates method order. A random tenant scopes cleanup; no global truncation or schema changes. A missing row, mismatched payload, partial SQL failure or incomplete report fails the command. Raw per-call latency samples are in JSON; medians summarize repetitions. Historical captures are retained below; their broker durability correction is documented with the results.

Run `make bench-pipeline` for three measured windows after warm-up. `BENCH_FLAGS='-pattern burst -rate 1000 -duration 10s -batch 100 -concurrency 8'` offers the same total event count in the first quarter of each window. Steady load spreads it across the whole window. Rates are targets; inspect attempted events, queue misses, late schedules, HTTP failures and resource snapshots before interpreting a run.

HTTP acknowledgement latency is one sample per successful request. Storage latency is one sample per observed event: API receipt to first SQL observation after commit, including polling and query delay. Receipt and observation use the deployment VM clock: PostgreSQL clock_timestamp is queried after reading committed rows; host time schedules the load. The clock probe and observer transaction overhead are reported, not subtracted. ObserverLatePolls counts sampling-start gaps above twice the requested polling interval; ObserverMaxGapNS retains the largest gap. It is an upper bound, not a physical commit timer. Its population is the pending observer's ObservedEvents; rows first found by the final independent check do not acquire invented latency samples. VerificationNS records that final query's overhead, which remains in the total rate denominator. Persisted throughput divides independently verified unique events by the complete window plus request tail and observation/final-verification drain; acceptance throughput counts only acknowledgements completed inside the requested window. Acknowledgements after that window and requests that only start after it are separate counts. Raw samples and backlog observations are retained. Acknowledged missing IDs, unexpected IDs or duplicate physical rows fail reconciliation. Ambiguous HTTP delivery is reported separately; an ambiguous ID absent at the observation deadline remains unknown, not proven undelivered. Outcomes with generator misses, late schedules, rejections, ambiguous delivery or request/ACK tails are labelled limited rather than attained load or capacity. The driver does not retry requests.

The CLI requires committed source and records that revision, image content IDs/repository digests, database durability/index settings, complete topic configuration, host/VM versions and coarse Docker resource snapshots. It removes only its owned stack. Local credentials remain in a private ignored run directory; public reports contain a whitelisted configuration. Database resets use a random tenant; pipeline cleanup names only its generated IDs and event type and refuses pre-existing namespace/ID collisions. Each case uses a recorded namespace to isolate IDs and event type. A temporary pending-ID relation observes committed rows without a timestamp cursor; an independent final ID/content query checks the result. Varied payload profiles use reproducible ASCII entropy at the exact requested encoded length; fixed/empty profiles retain legacy repeated padding for older single-run comparisons. Both run on the same host as their generator and observer. These experiments do not estimate maximum capacity or cloud cost.

[Recorded local experiments](results/local-2026-11-03/README.md) include every
repetition, exact configuration and the observed COPY/staging disadvantage.
The optional figure generator is `python3 benchmarks/plot_results.py` with
matplotlib installed; it reads these captures and writes docs/assets/experiments.svg.


**Durability correction:** the historical pipeline captures used a broker with
process-level fsync bypass enabled by dev-container mode. They do not establish
performance for the corrected configuration. The current stack explicitly disables
that bypass, and new manifests record the effective running-process setting.
Rerun before making deployment-throughput claims; do not rewrite old raw samples.
