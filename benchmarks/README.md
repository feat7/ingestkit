# Experiments

Run `make bench-db` with Docker running. Each invocation creates and destroys its own local stack, prints an evidence directory, and writes every measured repetition to JSON/CSV. No external database is accepted by the CLI.

Defaults: 10,000 submitted events, 256-byte JSON payload, 10% duplicate submissions, batch size 100, three repetitions after warm-up. Override via `BENCH_FLAGS='-count 1000 -batch 50 -duplicates 0'`. Single INSERT commits every row; batch INSERT and the production COPY/staging/merge store commit every batch. All use the same table, indexes, conflict policy and database settings. These transaction boundaries are part of the experiment.

Workload generation, cleanup and exact ID/payload verification are outside the timer. Each repetition rotates method order. A random tenant scopes cleanup; no global truncation or schema changes. A missing row, mismatched payload, partial SQL failure or incomplete report fails the command. Raw per-call latency samples are in JSON; medians summarize repetitions. Performance evidence will be added after the harness is committed.
