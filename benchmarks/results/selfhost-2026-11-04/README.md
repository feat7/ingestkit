# Corrected self-hosted ingestion study

Measured 4 November 2026 from clean source `213e4cc50523882f0d0cf7533d064494387d3207`. The deployed consumer uses atomic **batch INSERT**. The explicit COPY/staging/merge reference remains independently implemented in the database comparison. The [complete earlier COPY-writer study](../selfhost-copy-2026-11-04/README.md) motivated that change; its bytes and limits are retained.

## What this capture establishes

All 25 registered configurations completed with three measured repetitions: 99 database method/repetition rows, 33 HTTPS load windows and 9 recovery repetitions. Nine configurations contain reported limits. Every broker-acknowledged ID reconciled with independently verified stored content; no accepted missing IDs, unexpected IDs or duplicate physical rows were found. Ambiguous HTTP outcomes are a separate population and are **not** counted as successful delivery.

The independent [auditor](../../audit_study.py) verified registry controls, method/repetition counts, immutable artifact hashes and bounds, raw timing populations, nearest-rank p50/p95/p99, actual request/ACK windows, ID sets, rates, summaries, broker/topic/PostgreSQL durability and the trusted HTTPS route. An exact scan against this owned run’s generated credentials passed.

## Database comparison

5,000 submissions per method/repetition, rotated method order, varied deterministic JSON payloads. Generation, warm-up, reset and exact verification are outside elapsed time. Single INSERT commits each row; batch INSERT and COPY/stage/merge commit each batch against the same indexed table and first-content conflict policy. Values below are **median seconds [min–max] across all three repetitions**.

| Case | Single INSERT | Batch INSERT | COPY + merge |
| --- | ---: | ---: | ---: |
| db-batch-1 | 5.023 [4.936–7.941] | 5.088 [4.576–5.245] | 21.302 [21.177–22.681] |
| db-batch-10 | 4.522 [4.443–4.821] | 0.568 [0.524–1.255] | 2.144 [1.996–2.291] |
| db-batch-50 | 4.868 [4.694–10.170] | 0.193 [0.175–0.317] | 0.603 [0.546–0.639] |
| db-batch-100 | 6.510 [4.710–8.375] | 0.155 [0.121–0.247] | 0.486 [0.212–0.523] |
| db-batch-500 | 7.059 [6.646–7.221] | 0.085 [0.077–0.142] | 0.112 [0.111–0.118] |
| db-batch-1000 | 6.378 [6.155–9.558] | 0.081 [0.074–0.082] | 0.088 [0.075–0.108] |
| db-payload-1024 | 6.861 [5.654–10.520] | 0.195 [0.146–0.240] | 0.406 [0.274–0.416] |
| db-payload-16384 | 13.877 [7.016–14.818] | 1.006 [0.761–1.083] | 1.063 [1.062–1.115] |
| db-duplicates-10 | 5.675 [5.055–9.249] | 0.116 [0.115–0.212] | 0.302 [0.244–0.377] |
| db-duplicates-50 | 4.253 [3.998–4.346] | 0.096 [0.091–0.198] | 0.263 [0.253–0.509] |
| db-duplicates-90 | 1.394 [1.198–2.531] | 0.064 [0.056–0.080] | 0.229 [0.219–0.423] |

Batch INSERT had the lower median than COPY/staging/merge in all eleven configurations in both studies. Individual repetitions overlap or reverse that ordering in some larger-batch/payload cases. This supports the simpler default here, not a universal COPY threshold. No adaptive cutoff, UNNEST comparison or typed-schema speed claim is inferred. Duplicate configurations insert fewer unique rows; submitted/s differs from unique inserted/s.

## HTTPS pipeline

Ten-second windows after two-second warm-up; 50 ms target SQL polling; bounded 30-second observation drain; private-CA HTTPS with certificate verification. Default controls: 1,000 offered events/s, batch100, 32 clients, 256B varied payload. Larger payloads use batch10, with the 256B/batch10 case as their matched control. Burst offers the same total in the first quarter of the window.

Counts are per repetition in order1/2/3. ACK p95 is per successful HTTP request; SQL p95 is API receipt to first observed committed row per observed event, including polling/query delay. The two latency populations differ.

| Case | Attempted | Broker acknowledged | Verified stored | ACK p95 ms | SQL p95 upper bound ms |
| --- | --- | --- | --- | --- | --- |
| pipeline-batch-1 | 10000 / 9949 / 9377 | 10000 / 9949 / 9377 | 10000 / 9949 / 9377 | 17.38 / 20.53 / 31.92 | 75.82 / 91.93 / 239.38 |
| pipeline-batch-10 | 10000 / 10000 / 10000 | 10000 / 10000 / 10000 | 10000 / 10000 / 10000 | 21.65 / 21.97 / 118.65 | 97.96 / 104.65 / 302.32 |
| pipeline-batch-100 | 10000 / 10000 / 10000 | 10000 / 10000 / 10000 | 10000 / 10000 / 10000 | 40.19 / 41.28 / 42.92 | 101.61 / 103.08 / 102.86 |
| pipeline-rate-100 | 1000 / 1000 / 1000 | 1000 / 1000 / 1000 | 1000 / 1000 / 1000 | 57.13 / 53.51 / 64.47 | 97.18 / 92.49 / 95.83 |
| pipeline-rate-5000 | 50000 / 50000 / 50000 | 50000 / 50000 / 50000 | 50000 / 50000 / 50000 | 27.90 / 27.97 / 32.61 | 97.65 / 99.18 / 177.82 |
| pipeline-rate-10000 | 100000 / 100000 / 100000 | 100000 / 99000 / 100000 | 100000 / 99000 / 100000 | 30.60 / 30.25 / 27.58 | 156.22 / 173.50 / 134.91 |
| pipeline-payload-1024 | 10000 / 10000 / 10000 | 10000 / 10000 / 10000 | 10000 / 10000 / 10000 | 33.32 / 74.40 / 25.10 | 130.13 / 255.11 / 129.98 |
| pipeline-payload-16384 | 10000 / 10000 / 10000 | 10000 / 9840 / 10000 | 10000 / 9840 / 10000 | 49.53 / 60.63 / 42.91 | 251.74 / 270.53 / 179.64 |
| pipeline-burst | 10000 / 10000 / 10000 | 10000 / 10000 / 10000 | 10000 / 10000 / 10000 | 28.23 / 29.19 / 28.97 | 78.95 / 81.47 / 80.47 |
| pipeline-concurrency-8 | 4680 / 4644 / 4492 | 4680 / 4644 / 4492 | 4680 / 4644 / 4492 | 20.81 / 20.87 / 22.24 | 72.65 / 71.46 / 90.68 |
| pipeline-concurrency-64 | 10000 / 10000 / 9938 | 10000 / 10000 / 9938 | 10000 / 10000 / 9938 | 19.58 / 21.20 / 37.42 | 86.18 / 109.43 / 225.58 |

The batch100 baseline, 100/s case and first-quarter burst completed without reported limits. Other cases retain generator misses, late schedules, request/ACK tails, observer delays or ambiguous delivery. A finite-window tail is not by itself evidence of saturation. Lower attempted counts mean the driver did not generate the target; they do not mean the collector lost those unattempted events.

At 10,000 offered events/s, repetition2 retained 1,000 ambiguous IDs absent at its final deadline. The 16KiB case’s repetition2 retained 160 such IDs. Absence at that deadline means delivery remains unknown, not proved never delivered. Reuse those IDs on retry. No ambiguous ID is converted into an acknowledgement or a fabricated latency sample.

In-window ACK rate divides only acknowledgements completed inside the target window by that window. Stored rate divides independently verified rows by the full window, request tail, observation drain and final independent verification. A final census can find a row without yielding a pending-observer latency sample; `ObservedEvents` and `VerificationNS` expose the distinction. Inspect all rate denominators and limit reasons in [index.json](index.json). No maximum capacity or cloud-cost claim follows.

## Availability experiments

Each row below represents three repetitions, separate from throughput claims. Actual fault request/completion timestamps are retained in raw captures.

| Experiment | Controls | Verified outcome |
| --- | --- | --- |
| PostgreSQL | Stop requested at3s; start requested at8s during15s at offered500 events/s | Every accepted ID/content reconciled after recovery; SQL-observer outage/gap remains explicit |
| Consumer | KILL requested at3s; restart requested at5s during the same workload | Every accepted ID/content reconciled after group recovery; actual Docker timings retained |
| Outbox + broker | 100 persisted application events; broker stopped for8s after stop completion; failed attempt exceeds5s | Pending IDs survived a new dispatcher; replay yielded100 accepted/stored unique IDs each repetition |

These availability results do not prove an arbitrary kill landed inside the SQL-commit/offset window. The separate deterministic process-kill regression pauses exactly there, verifies an uncommitted offset, and replays to one stored row. SQL gaps cannot reveal precise commit times. Docker stop/start completion is a control observation, not an exact service-availability clock.

## Environment and reproducibility

[manifest-1.json](manifest-1.json) records actual versions/image IDs/digests, running broker process controls, source/host/VM metadata, topic configuration, database durability/indexes and trusted TLS protocol. Broker1core/1GiB, one replica, seven-day log retention; process fsync bypass false and topic write caching false. PostgreSQL18.6 fsync, synchronous commit and full-page writes on. API limit10,000 requests/s, info logging. Local TLS trust was verified; public ACME issuance was not tested.

The M1 Max physical host had10 logical CPUs/68,719,476,736 bytes RAM; OrbStack’s Linux Docker VM exposed10 CPUs/16,818,991,104 bytes RAM. The generator and SQL observer ran on that same physical host alongside existing workloads. Only owned containers were controlled. Resource snapshots are coarse observations, not a calibrated bottleneck or isolated-disk benchmark. One broker/host does not protect against host/disk loss or retention expiry.

Run from the repository root:

~~~sh
python3 benchmarks/audit_study.py benchmarks/results/selfhost-2026-11-04
# New timing run: commit source and choose a fresh output directory.
make bench-study STUDY_FLAGS='-out benchmarks/results/my-study -seed 42'
~~~

See the [study guide](../../README.md) for controls, prerequisites, resume and artifact bounds. The current source can audit these immutable captures; reproducing the exact measured implementation requires the recorded source revision. Changes to docs/evidence after that revision do not retroactively remeasure it.

[index.json](index.json) contains readable controls, summaries, statuses and SHA256 references; [summary.csv](summary.csv) lists every case. [journal.json](journal.json) retains progress/attempts. Raw gzip files under `raw/` have zero timestamps and include all repetitions, request/event timing samples, backlog observations and compact ID populations. Metadata and coarse resources are independently checksummed. Private keys/config/CA/logs remain in the ignored owned run directory. No failed/limited case was discarded.
