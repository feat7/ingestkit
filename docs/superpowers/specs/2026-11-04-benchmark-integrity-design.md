# Reliable measurements and repository cleanup

Status: proposed for review; this document does not authorize implementation.
Native execution remains the user's selected implementation method.

## Purpose

Find which connection or concurrency limits actually constrain this deployment,
repair the benchmark's measurement failures, and remove obsolete or misleading
results from the public repository. Preserve enough evidence outside the checkout
to explain every removal. Keep valid overload results in the replacement study.

The current local investigation does not establish maximum server capacity. With
ten HTTP workers and batches of 100, mean acknowledgement times of 48.9–51.2 ms
predict approximately 19.6k–20.4k events/s when those workers stay occupied. The
observed 19.5k–20.3k acknowledgement rates closely match that driver limit. This
is strong evidence of a test constraint, not a measured server ceiling.

## Approach

Use measurement repair and removal of affected public claims first, followed by
a controlled connection study. Raising every limit together would prevent causal
attribution. Deleting only slow results would leave misleading evidence. Keeping
an expanding public historical-results archive would work against the requested
cleanup. The proposed archive therefore lives outside the Git checkout.

This is a proposal based on the user's requested investigation and cleanup, not
a claim that the detailed implementation choices have already been approved.

## What is known, and what still needs measuring

| Layer | Current evidence | What the next run must capture |
|---|---|---|
| Driver | Explicit worker limits of 8, 10, 64, or 128; HTTP/1.1 in captured runs | Active requests, active/open TCP connections, connection acquisition time, dial/reuse counts, scheduling/queue delays |
| Go HTTP transport | `MaxIdleConnsPerHost` retains idle connections; `MaxConnsPerHost=0` permits unlimited connections | Effective active and idle limits; protocol; peak actual connections; errors while connecting |
| Caddy | No explicit upstream connection cap in either Caddyfile; documented default has no cap | Effective adapted configuration, upstream requests/connections/errors, runtime process limits |
| API | No explicit Fiber concurrency override; pinned Fiber 2.52.9 defaults to 262,144 | Effective configured value, file-descriptor limit and usage, accept/publish errors |
| Producer | 1,000 buffered records and 64 MiB by default | Buffered records/bytes, publish-error class, publish latency; these are record limits, not TCP limits |
| Consumer | Processes partition batches sequentially; SQL batch maximum 500 | Actual batch sizes, write/offset-commit timing, active DB acquisitions and pool waits |
| pgx pools | Default maximum is greater of four or `runtime.NumCPU()`; no explicit override | Actual configured maximum per process, acquired/idle/total connections, acquisition waits |
| PostgreSQL | Captured `max_connections=100` | Actual sessions by test role/application, pool waits, connection refusals, database wait events |
| Host/VM | Current shell soft FD limit is 1,048,575; this was not recorded in old driver processes | Actual running-process soft/hard limits, open descriptors, socket errors, ephemeral-port range/state where available |

Go defaults are checked against the installed Go source; pgx/Fiber defaults are
checked against pinned module source. Caddy's documented connection default:
[Caddy HTTP transport reference](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy#the-http-transport).
An unavailable runtime measurement must remain unavailable, never become zero.
HTTP/2 can multiplex requests, so request concurrency and TCP connection counts
must remain distinct even if the initial comparison explicitly uses HTTP/1.1.

## Measurement contract

- Separate offered, scheduled, started, acknowledged, and independently verified
  stored events. Record HTTP request counts and events/request alongside them.
- Separate worker limits, active TCP limits, idle retention, DB pool sizes,
  database-wide connection limits, and producer record buffers.
- HTTP result collection and the offered-load clock must not wait on SQL probes.
  Use one cancellable observer worker with at most one query in flight and a
  bounded result channel; coalesce ticks rather than queueing probe work.
- An observer failure must not erase valid HTTP completion evidence or successful
  final ID/payload verification. Visibility metrics become unavailable/partial
  with an explicit status and sample count, not zero or a false delivery loss.
- Report delivery verification, load completeness, and observation health
  separately. No single `correct=true` or requested rate is a capacity claim.
- Keep harness bookkeeping time, first observed complete storage, final verification
  time, and subsequent offset waiting separate. Never rename total verification
  completion rate as SQL write capacity.
- Publish schema 2 captures for the replacement results. Old schema 1 bytes stay
  unchanged in the external archive and can be read with their pinned old source.
- Do not disable fsync, synchronous commit, full-page writes, TLS verification,
  replay protection, or the write-before-offset rule to improve numbers.
- No production optimization is justified until the relevant limit/wait is
  observed. Larger DB pools do not create consumer write parallelism.

## Cleanup policy

Archive the full old bundles before removing them, including valid database or
recovery subsets. They combine several kinds of evidence, and splitting or
editing them in place would break their provenance. Removing a superseded bundle
does not mean every datum in it was wrong.

Default archive location: `/Users/vinay/Workspace/personal/ingestkit-evidence/`
with a new `2026-11-04-<archive-id>/` child. Use an owner-only directory, a byte
hash manifest, source revisions and removal reasons. Never overwrite an archive,
follow symlinks, or print generated deployment credentials. Verify every copied
file before removing its original. Preserve Git history; no history rewrite.

| Current path | Planned disposition |
|---|---|
| `benchmarks/results/local-2026-11-03/` | Archive externally and remove all 21 tracked files; includes legacy broker settings unsuitable for durable-ingestion claims |
| `benchmarks/results/selfhost-copy-2026-11-04/` | Archive externally and remove all 31 tracked files; retire combined schema 1 publication bundle |
| `benchmarks/results/selfhost-2026-11-04/` | Archive externally and remove all 31 tracked files; replace combined schema 1 publication bundle |
| `docs/assets/experiments.svg` | Retire obsolete experiment image |
| `docs/assets/{database-batching,pipeline-load,recovery}{,-mobile}.svg` | Retire all six old measured charts and their public references |
| `docs/figures/{verification,story-verification}.json` | Archive old receipts intact; replace with one truthful manifest covering retained assets and new outputs |
| `benchmarks/plot_results.py` | Archive/replace the old study-specific plot generator after its documentation references are removed |
| The seven pre-existing files under `docs/superpowers/` | Archive old designs, plans, reviews and launch drafts externally after extracting any still-required checks |
| `tmp/copy-investigation/`, `tmp/capacity-investigation/` | Move externally after source/hash review; these were ignored local probes and must never be promoted wholesale to published results |
| Obsolete `.archify/` previews/exports and `tmp/runs/` evidence | Inventory by exact path, preserve needed source/provenance externally, then remove only confirmed owned and unused paths |
| API, deployment, operations, integration and example guides | Keep; repair references and claims rather than deleting working instructions |
| Event journey, worker replay, deployment and delivery diagrams, with source JSON and required generators | Keep while their source-backed behavior remains accurate; no replacement decorative diagrams in this work |

Do not delete an active or unrelated worktree, running service, private live
deployment, application outbox, or unrelated project data. The current approved
plan/design remain available throughout execution; archive them at delivery if
the public repo would otherwise retain redundant workflow documents.

## New experiment sequence

1. Calibrate the driver against a clearly labelled TLS acknowledgement sink,
   independently of the ingestion services. It must account for every planned
   request and expose its own connection/scheduling failures.
2. On the local owned production-profile stack, fix HTTP/1.1, 1 KiB payloads,
   100 events/request, 128 workers, and a 15k events/s target. Vary only
   `MaxConnsPerHost`: 4, 8, 16, 32, 64, 128, then 0 (unlimited control).
   Keep idle retention fixed at 128. Use three ten-second windows per setting.
3. Next screen targets 15k, 30k, and 50k
   with the same payload/batch and three ten-second windows. Fix the active cap
   at 128 and workers/idle retention at 128 for this rate cohort. Reuse the
   15k/cap-128 sweep baseline explicitly; add only the 30k and 50k cases. Keep
   all overload and missed-load records. Change a server or pool limit only in
   a separately named comparison justified by observed waits/errors.
4. Use a separate matched payload cohort: 256 bytes, 1 KiB, and 16 KiB; ten
   events/request; 1,000 events/s; three ten-second windows; workers, active
   connection cap and idle retention all 128. Ten 16 KiB payloads plus envelopes
   fit the API's existing 1 MiB request limit; 100 would not. Keep the request
   limit unchanged. This is the plan author's proposed control, not a new user
   requirement. Do not compare its results directly with the 100-event main
   cohort or extrapolate them to 50k/s.
5. Publish only verified local screening results under one new schema 2 bundle.
   Do not claim sustained capacity from these windows.

The registered connection suite therefore contains 12 unique cases: seven
connection caps, two additional high-rate cases, and three matched payload
cases. Transport cap 128 at 15k is one measurement reused by two views, not
two secretly selected results. If the observed driver cannot offer a target,
the case remains a labelled driver-limited result, not a server-capacity limit.

A later sustained-capacity milestone requires a dedicated deployment, a separate
load generator and continuous runs of at least 60 seconds. At 50k/s that is three
million events, beyond the current one-million-event guard and potentially the
artifact limits. That milestone needs a bounded/streaming dataset and segmented
captures designed separately; do not just raise the guard or insert pauses
between short windows and call them a continuous run. It is a publication gate,
not a reason to postpone measurement repair or repository cleanup now.

## Completion

The public tree has no retired result directories, broken links, stale measured
charts or unsupported capacity/COPY claims. It contains one independently audited
replacement screening bundle, including valid negative results and metric validity.
Runtime constraints and real connection/pool peaks are recorded. The observer
regressions and truthful failure-state tests pass. Native execution, required
checks and a final independent review precede the existing PR/merge workflow.
Full article storytelling and a sustained-capacity headline remain later work.
