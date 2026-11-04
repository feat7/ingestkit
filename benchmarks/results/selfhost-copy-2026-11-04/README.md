# Results with the original COPY writer

This complete study ran on 4 November 2026 from clean source `dccf4b3`, before
the deployed worker changed to batch INSERT. It uses the corrected HTTPS
self-hosted setup. The [second study](../selfhost-2026-11-04/README.md) measures
the changed writer. Keep this first capture as the comparison that led to it.

All 25 cases completed with three measured runs each: 99 database method/run
rows, 33 HTTP load windows and 9 recovery runs. Nine cases report limits. Every
acknowledged event ID was found with its expected stored content. Independent
Python checks verified raw samples, nearest-rank percentiles, request/ID sets,
rates, summaries, checksums, case controls, durability settings and HTTPS. A scan
for the run's exact generated credentials found none in the captures. All limits
remain in the index and raw files; these are not maximum-capacity measurements.

Batch INSERT had a lower median elapsed time than COPY/staging/merge in all 11
database configurations. That result led to a separate writer change and a new
full study. It does not establish a general ranking of PostgreSQL COPY.

[manifest-1.json](manifest-1.json) records source, images, runtime and host settings.
The sender and SQL observer shared an M1 Max host with its Docker VM and other
workloads; only this run's containers were controlled. PostgreSQL fsync and
synchronous commit were on. The broker's process bypass and topic write caching
were false. A single broker replica does not recover data lost with the only host
or disk. Private-CA HTTPS was verified; public certificate issuance was not tested.

[index.json](index.json) contains all settings, summaries, limits and raw-file
references. [summary.csv](summary.csv) lists the cases; [journal.json](journal.json)
records progress. Run `python3
benchmarks/audit_study.py benchmarks/results/selfhost-copy-2026-11-04` from the
repository root for an independent read-only audit. Raw gzip timestamps are zero
and files have SHA-256 references. No failed or limited case was discarded.
