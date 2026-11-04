# Historical local experiments (3 November 2026)

These pipeline runs used Redpanda's dev-container mode. It started v26.2.6
with `--unsafe-bypass-fsync=true`, even though topic write caching was off.
The [startup-mode reference](https://docs.redpanda.com/streaming/current/reference/rpk/rpk-redpanda/rpk-redpanda-start/)
explains the separate process flag. Their timings describe that development setup
and cannot establish fsync-enabled broker throughput or deployment sizing.

The [replacement self-hosted study](../selfhost-2026-11-04/README.md) uses the
corrected configuration. These old raw captures stay unchanged. The PostgreSQL-only
comparison still has its recorded fsync/synchronous_commit settings.


Measured source: [deefc8162504fa482c19c63edda05c1c1a4c2ae7](https://github.com/feat7/ingestkit/tree/deefc8162504fa482c19c63edda05c1c1a4c2ae7). All four manifests record clean source at this revision. Subsequent validation/lifecycle fixes are regression-tested but these timing captures have not been rerun on those later revisions. The recorded revision identifies the implementation used for these samples.

Apple M1 Max, 10 logical CPUs, 64 GiB physical memory; OrbStack Docker VM with 10 CPUs and about 15.7 GiB reported memory. Go generator 1.26.5; container builds 1.26.10. PostgreSQL 18.6, Redpanda 26.2.6, one broker/replica, event topic three partitions, one consumer. PostgreSQL fsync/synchronous_commit/full_page_writes were on; topic write.caching was false, retention seven days. Exact versions, image hashes, indexes and topic settings are in each manifest. Host, generator, observer and services share resources. The manifest's generic caveat says “Docker Desktop”; its actual runtime fields identify OrbStack. The numbers below describe this local VM.

## Database methods

10,000 submissions, 256-byte JSON payloads, configured batch size 100, one complete batch of warm-up per method, three measured repetitions with rotated method order. Every repetition verified exact IDs, types, timestamps and JSON payloads. No generation, reset or verification inside the timer. Single INSERT commits every row; batch INSERT and COPY/stage/merge commit every batch. At that recorded revision, the COPY case calls the service's store.

| Method | No duplicates: median seconds | Submitted/s | 10% duplicates: median seconds | Submitted/s |
| --- | ---: | ---: | ---: | ---: |
| INSERT | 16.909 | 591 | 13.054 | 766 |
| Batch INSERT | 0.492 | 20,333 | 0.256 | 39,070 |
| COPY/stage/merge | 1.011 | 9,889 | 0.706 | 14,161 |

[Unique workload JSON](db-unique/database.json), [CSV](db-unique/database.csv), [manifest](db-unique/manifest.json). [Duplicate workload JSON](db-duplicates/database.json), [CSV](db-duplicates/database.csv), [manifest](db-duplicates/manifest.json).

The duplicate workload submitted 10,000 and inserted 9,000 distinct events each time. Its submitted/s includes ignored duplicates, not 10,000 new rows. The duplicate workload did less physical insertion and ran in a separate stack; these samples do not isolate a deduplication overhead percentage. Across unique repetitions, batch INSERT elapsed ranged 0.358–0.517s and COPY/merge 0.861–1.019s. At this small batch size, staging/merge lost to ordinary batched SQL. This is a workload-specific comparison, not a universal ranking or a reason to advertise COPY as faster.

## Whole pipeline

p95 is the time by which 95% of the relevant samples finished. HTTP timing
samples successful requests; storage timing samples observed events.

Each configuration: two-second warm-up, three ten-second measured windows, 10,000 unique events/window, 256-byte payload, HTTP batches of 100, eight workers. Steady offers 1,000 events/s throughout; burst offers the same count at 4,000 events/s during the first 2.5 seconds, then remains idle for the rest of the window. No retries in the load driver.

| Pattern / repetition | Accepted / stored | Missed / ambiguous / rejected | Stored/s including completion | HTTP ACK p95 ms | Storage visibility p95 ms |
| --- | --- | --- | ---: | ---: | ---: |
| Steady 1 | 10,000 / 10,000 | 0 / 0 / 0 | 995 | 27.9 | 73.1 |
| Steady 2 | 10,000 / 10,000 | 0 / 0 / 0 | 996 | 31.0 | 85.2 |
| Steady 3 | 10,000 / 10,000 | 0 / 0 / 0 | 996 | 27.4 | 72.7 |
| Burst 1 | 10,000 / 10,000 | 0 / 0 / 0 | 996 | 137.9 | 261.5 |
| Burst 2 | 10,000 / 10,000 | 0 / 0 / 0 | 996 | 18.1 | 68.7 |
| Burst 3 | 10,000 / 10,000 | 0 / 0 / 0 | 995 | 17.0 | 68.8 |

[Steady raw JSON](steady/pipeline.json), [CSV](steady/pipeline.csv), [environment](steady/manifest.json), [resource snapshots](steady/resources.json). [Burst raw JSON](burst/pipeline.json), [CSV](burst/pipeline.csv), [environment](burst/manifest.json), [resource snapshots](burst/resources.json).

Before cleanup, every acknowledged ID was checked against the IDs stored in PostgreSQL; the sets matched. Sorted unique ID hashes are saved alongside deterministic seeds; raw UUID arrays are omitted to keep the captures bounded. All latency samples are retained. No late schedules or queue misses were recorded. There were no accepted IDs still pending at the ten-second boundary; the final SQL confirmation adds approximately 41–55ms to the complete interval. The coarse sampled accepted-minus-visible peak was 881 in burst repetition 1, versus 36 and 100 in repetitions 2 and 3. This counts acknowledged events that the SQL observer had not yet seen. It does not measure the exact number of unread queue records. The cause of the first burst's latency variation is not established; these three repetitions are insufficient for a general latency guarantee.

HTTP ACK latency is per successful request, including client encoding/network and broker acknowledgement. Storage latency is per event, from API receipt to the end of the first SQL query that sees the committed row. It includes 50ms polling plus query time and is an upper bound, not the physical commit instant. Persisted/s uses the complete ten-second window plus request tail and final confirmation, so burst's idle time is included. It does not describe the active burst's processing capacity.

These experiments check a chosen sending rate and its saved events. They do not establish maximum capacity. Resource samples do not isolate the bottleneck. There is no cost/server sizing result, sustained saturation test, long-duration stability result or host/disk-loss experiment here. All four status files report successful scoped resource destruction. Recovery experiments are separate integration tests, not throughput samples.

## Reproduce

From the recorded revision with Docker running:

~~~sh
make bench-db BENCH_FLAGS='-count 10000 -batch 100 -duplicates 0 -out tmp/db-unique'
make bench-db BENCH_FLAGS='-count 10000 -batch 100 -duplicates 0.1 -out tmp/db-duplicates'
make bench-pipeline BENCH_FLAGS='-pattern steady -rate 1000 -duration 10s -batch 100 -concurrency 8 -out tmp/steady'
make bench-pipeline BENCH_FLAGS='-pattern burst -rate 1000 -duration 10s -batch 100 -concurrency 8 -out tmp/burst'
~~~

Run sequentially. Compare your own raw repetitions and environment; do not expect a laptop/VM result to transfer to a cloud instance. Reports contain no API key or credentialled database URL. Each stack's private run.env remains ignored under tmp/runs.
