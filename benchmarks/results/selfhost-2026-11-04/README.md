# Results with the batch-INSERT writer

Measured on 4 November 2026 from clean source `213e4cc50523882f0d0cf7533d064494387d3207`.
The deployed worker uses atomic batch INSERT. The database comparison still has
an independent COPY/staging/merge implementation. The [earlier complete study](../selfhost-copy-2026-11-04/README.md)
led to the writer change; its captures and limits remain unchanged.

## What was checked

All 25 cases completed with three measured runs each: 99 database method/run
rows, 33 HTTPS load windows and 9 recovery runs. Nine cases report limits. Every
ID acknowledged by the queue was found with its expected content in the independent
storage check. There were no missing acknowledged IDs, unexpected IDs or duplicate
physical rows. Requests with uncertain outcomes are counted separately and never
become successful deliveries in the report.

The [independent auditor](../../audit_study.py) checked registered settings,
method/run counts, file checksums and size limits, raw timing samples, nearest-rank
p50/p95/p99, request/ACK windows, ID sets, rates, summaries, broker/topic/PostgreSQL
durability settings and trusted HTTPS routing. A scan for this run's exact generated
credentials found none in the published captures.

## Which database write was faster?

Each method submitted 5,000 varied, reproducible JSON events per run. Method order
rotated. Generation, reset, warm-up and exact verification happened outside the
write timer. Single INSERT committed each row; batch INSERT and COPY/stage/merge
committed each batch to the same indexed table, preserving the first stored content
for repeated IDs. Each cell is median seconds followed by the range of all three runs.

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

Batch INSERT had a lower median than COPY/staging/merge in all eleven cases in
both studies. Some individual runs overlapped or reversed that order, particularly
with larger batches/payloads. The result supports the simpler writer for this
workload. It establishes no universal COPY threshold, adaptive cutoff, UNNEST
comparison or typed-schema speed advantage. Duplicate cases write fewer new rows;
submitted/s therefore differs from unique inserted/s.

## What reached storage over HTTPS?

The load window was ten seconds after two seconds of warm-up. SQL polling aimed
for every 50 ms, with a bounded 30-second observation period afterward. HTTPS used
a verified private CA. Defaults were a target of 1,000 events/s, batches of 100,
32 clients and varied 256-byte payloads. Larger payloads used batch 10 with the
256-byte/batch-10 case as their control. The burst sent the same total in the
first quarter of the window.

Read counts in run order 1/2/3. ACK p95 samples successful HTTP requests. SQL p95
samples observed events from API receipt until the first completed query that
sees their committed row, including polling/query delay. These are different
sample sets. p95 is the time by which 95% of the relevant samples finished.

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

The batch-100 baseline, 100/s case and burst finished without reported limits.
Other cases retain missed sending schedules, late requests/responses, observer
delays or uncertain delivery. A response outside the window alone does not show
saturation. Lower attempted counts mean the driver did not send its full target;
the collector cannot lose an event that was never submitted.

At a target of 10,000 events/s, run 2 left 1,000 uncertain IDs absent at the final
check. The 16 KiB case left 160 such IDs in run 2. Delivery remains unknown at that
deadline; it is not proved impossible later. Retry the same IDs. They contribute
neither an acknowledgement nor an invented latency sample.

In-window ACK rate counts only responses completed in the target window. Stored
rate divides verified rows by the full window, unfinished requests, observation
and final verification. Rows found only in the final query may have no observation
latency sample; `ObservedEvents` and `VerificationNS` make that distinction visible.
[index.json](index.json) records the denominators and each limit. These runs do not
establish maximum capacity or cloud cost.

## Did events survive the outages?

Each experiment ran three times. Raw captures retain the actual fault request
and completion times.

| Experiment | Controls | Verified outcome |
| --- | --- | --- |
| PostgreSQL | Stop requested at3s; start requested at8s during15s at offered500 events/s | Every accepted ID/content reconciled after recovery; SQL-observer outage/gap remains explicit |
| Consumer | KILL requested at3s; restart requested at5s during the same workload | Every accepted ID/content reconciled after group recovery; actual Docker timings retained |
| Outbox + broker | 100 persisted application events; broker stopped for8s after stop completion; failed attempt exceeds5s | Pending IDs survived a new dispatcher; replay yielded100 accepted/stored unique IDs each repetition |

The exact SQL-commit/offset crash test is separate: it pauses after SQL commits,
checks that the queue position is still unsaved, then restarts and verifies one
row after replay. An arbitrary restart would not prove it hit that gap. SQL
observation gaps cannot reveal precise commit times, and Docker stop/start
completion is a control timestamp rather than an exact availability transition.

## Environment and reproduction

[manifest-1.json](manifest-1.json) records versions, image IDs/digests, actual
broker process settings, host/VM, topics, database durability/indexes and verified
TLS. The broker had one core, 1 GiB, one replica and seven-day retention, with
process fsync bypass and topic write caching both false. PostgreSQL 18.6 had fsync,
synchronous commit and full-page writes on. The API limit was 10,000 requests/s
with info logging. Local HTTPS trust was tested; public ACME issuance was not.

The M1 Max host had 10 logical CPUs and 68,719,476,736 bytes RAM. Its OrbStack
Linux Docker VM exposed 10 CPUs and 16,818,991,104 bytes RAM. The generator and
SQL observer shared the physical host with existing workloads. Only this run's
containers were controlled. Coarse resource snapshots do not isolate a bottleneck
or measure disk performance. One broker/host does not protect against disk loss,
host loss or retention expiry.

Run from the repository root:

~~~sh
python3 benchmarks/audit_study.py benchmarks/results/selfhost-2026-11-04
# New timing run: commit source and choose a fresh output directory.
make bench-study STUDY_FLAGS='-out benchmarks/results/my-study -seed 42'
~~~

The [experiment guide](../../README.md) covers prerequisites, controls, resume
and file limits. Current source can audit these captures. Repeating the exact
measured implementation requires its recorded revision; later documentation or
evidence changes do not remeasure it.

Use [index.json](index.json) for controls, results, statuses and SHA256 references;
[summary.csv](summary.csv) lists every case and [journal.json](journal.json) keeps
attempt history. Raw gzip files in `raw/` have zero timestamps and contain all
runs, timing samples, backlog observations and compact ID sets. Metadata and
coarse resources have independent checksums. Private keys/config/CA/logs stay in
the ignored run directory. Failed or limited cases were not discarded.
