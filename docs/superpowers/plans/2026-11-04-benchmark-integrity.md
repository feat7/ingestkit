# Benchmark Integrity and Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. The user previously selected native execution. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** Proposed for review. No measurements, deletions or implementation are performed by writing this plan.

**Goal:** Identify actual connection constraints, repair measurement correctness, and replace obsolete public evidence with one audited local screening study.

**Architecture:** Keep the production delivery policy and writer intact. Decouple SQL observation from HTTP bookkeeping, instrument effective connection/resource limits, run controlled comparisons, and withdraw old public bundles through a verified external archive.

**Tech Stack:** Go, net/http/httptrace, pinned pgx/Fiber/franz-go, Docker Compose, PostgreSQL 18.6, Redpanda, Python standard-library auditing, optional matplotlib for measured plots.

**Spec:** `docs/superpowers/specs/2026-11-04-benchmark-integrity-design.md`

## Global constraints

- Native execution; no implementation before plan review.
- No production optimization is justified until the relevant limit/wait is observed.
- Separate offered, scheduled, started, acknowledged, and independently verified stored events.
- Publish schema 2 captures; preserve old schema 1 bytes unchanged outside the checkout.
- Never disable fsync, synchronous commit, full-page writes, TLS verification, replay protection, or write-before-offset progress.
- Archive to `/Users/vinay/Workspace/personal/ingestkit-evidence/2026-11-04-<archive-id>/` with byte verification before deletion; preserve Git history.
- No sustained-capacity headline from local ten-second screens. The separate long-run milestone in the spec remains a publication gate.

## Review focus

1. SQL probes slow, fail, or finish after all HTTP work: request accounting must still complete promptly and final delivery verification must remain independent. Task 2.
2. HTTP/2 multiplexing, idle retention, transport waiting and worker limits: never label them as the same connection limit. Task 3.
3. Pool exhaustion or missing runtime permissions: record waits/unavailability; never fabricate zero usage or automatically raise limits. Task 3.
4. Symlinks, duplicate archive targets, active resources and unrelated files: fail closed before deletion. Task 1 and Task 7.
5. Unsent, uncertain, partly published or observer-limited work: preserve valid observations without turning them into achieved capacity or false delivery loss. Tasks 2, 4 and 6.

## Task 1: Withdraw superseded evidence through a verified archive

**Files:** Create `scripts/archive_benchmark_evidence.py` and `scripts/test_archive_benchmark_evidence.py`; modify the reader references listed in Task 6. Remove the three result directories and seven measured SVGs listed in the spec only after verified archival. Do not move active diagnostic source before its useful behavior is captured in tests.

**Interface:** `archive_paths(repo_root: Path, archive_root: Path, relative_paths: list[str], *, apply: bool) -> dict`. Default is inventory/dry-run. `--apply` creates a new archive, records original paths, source SHA, sizes, SHA-256 and reasons, verifies destination bytes, then removes only listed originals. Detect Git-tracked removals for the eventual commit; never rewrite Git history.

Use two internal phases: copy and verify **all** listed files before removing
**any** source. A late copy failure leaves every original in place. Record the
canonical checkout root in each manifest. Tracked retirements apply in the
implementation worktree; ignored probes from the primary checkout require a
separate exact-path inventory rooted at that checkout. Never recurse through
`.worktrees/` or treat the primary checkout's tracked files as cleanup targets.

- [ ] Write failing tests: `test_verified_copy_required_before_remove`, `test_symlink_or_escape_refused`, `test_existing_archive_refused`, `test_late_copy_failure_preserves_all_sources`, `test_only_listed_owned_paths_removed`, and `test_manifest_contains_no_file_contents`.
- [ ] Run `python3 -m unittest discover -s scripts -p 'test_archive_benchmark_evidence.py'`; confirm the new tests fail for the intended behavior before implementation.
- [ ] Implement the archive interface and a dry-run inventory from the spec's exact path list. Require an explicit inactive-resource check before any later private run-directory cleanup.
- [ ] Run tests, inspect the dry-run manifest, apply the tracked-result/chart archive, and independently compare every source/destination hash before accepting removals.
- [ ] Replace numeric result paragraphs/links with a short measurement-revalidation notice; retain working setup, safety boundaries and integration instructions. Commit this self-contained withdrawal so the public tree need not retain known misleading material while repairs proceed.

## Task 2: Repair observer/bookkeeping isolation and result validity

**Files:** Modify `internal/benchmark/pipeline.go`, `pipeline_test.go`, `observer.go`, `observer_integration_test.go`, `verification.go`, `results.go`, `results_test.go`, `study.go`, `study_test.go`, `artifacts.go`, `benchmarks/audit_study.py`, `benchmarks/test_audit_study.py`.

**Interfaces:** Preserve `runPipelineSample(ctx, config, repetition, send, observe)` and `RunPipeline` call signatures. Add a single observer worker, cancellable/joined before closing its session, with at most one query in flight and one queued result. HTTP completions stay owned by the main accounting loop; the observer never mutates its maps. Emit schema 2 and explicit `load_status`, `delivery_status`, `observation_status`, nullable visibility metrics and sample counts. Use `met|limited|aborted`, `passed|failed|not_run`, and `complete|partial|failed|not_run`, respectively. Retire the ambiguous pipeline `PersistedEPS` headline field in schema 2; report verified completion duration separately from commit/acceptance rates.

- [ ] Write `TestSlowObserverDoesNotStarveHTTPResults`: use many immediately completed requests, a controlled slow observer and signalled channels; assert every completion is accounted before allowing another probe. Avoid relying on random select behavior or a microbenchmark wall-time threshold.
- [ ] Write `TestObserverFailurePreservesACKsAndFinalVerification`: force an observation error, then a matching independent final result; ACK counts remain valid, delivery passes, observation fails, visibility p95 is unavailable, command retains the measurement error.
- [ ] Write `TestCancellationStopsAndJoinsObserver`, `TestObserverHasOneInflightQuery`, `TestNoPollingAfterFinalRequiredObservation`, and `TestAmbiguousIDsKeepBoundedDeadlineWithoutFalseLoss`.
- [ ] Run those tests and observe the intended failures. Implement observation isolation and separate validity states, including recovery-mode reconnect behavior and zero-ACK/partial-publication paths.
- [ ] Update schema writers/auditor together. Add audit fixtures rejecting wrong denominators, mismatched request/event counts, zero substituted for failed visibility, altered ID hashes, and any conversion of `limited` into achieved capacity. Keep valid overload records auditable.
- [ ] Run `go test -race ./internal/benchmark` and `python3 -m unittest discover -s benchmarks -p 'test_audit_study.py'`; run targeted observer integration tests in the owned fixture using the existing integration script.
- [ ] Commit the measurement repair with a short regression explanation and the demonstrated 11.120 s versus 69.691 s example, without claiming a production performance fix.

## Task 3: Capture real connection and resource constraints

**Files:** Create `internal/benchmark/connections.go`, `connections_test.go`, `internal/telemetry/pool.go`, `pool_integration_test.go`, and `internal/runtimeinfo/{limits.go,limits_linux.go,limits_darwin.go,limits_other.go,limits_test.go}`; modify `manifest.go`, `pipeline.go`, `internal/telemetry/metrics.go`, `internal/messaging/publisher.go`, `publisher_test.go`, and `cmd/consumer/main.go`.

**Interfaces:** `NewConnectionRecorder() *ConnectionRecorder`, `(*ConnectionRecorder).WrapTransport(*http.Transport) *http.Transport`, and `(*ConnectionRecorder).Snapshot() ConnectionSnapshot`. Preserve CA verification and HTTP timeout settings. A snapshot records active requests separately from open TCP connections; configured active/idle caps; connection acquisition duration; dial/reuse/TLS/protocol observations; typed connection errors; and availability flags for process FD/port information. Platform-specific process-limit readers use build-tagged files. `(*Metrics).RegisterPool(*pgxpool.Pool)` exposes only numeric private metrics, not connection strings.

`runtimeinfo.CaptureProcessLimits() ProcessLimits` is shared by driver and service
instrumentation. Numeric limits distinguish known, unlimited and unavailable;
unavailable values remain null. Keep driver and consumer pool statistics separate.

- [ ] Write a real `httptest` transport test: 16 workers and `MaxConnsPerHost=1`, hold the first response, assert only one TCP connection and positive connection wait; release and assert all requests finish. Repeat with cap 8.
- [ ] Test that `MaxIdleConnsPerHost=2` permits more than two active connections. Test TLS/HTTP2 reporting separately so multiplexing cannot be mistaken for extra sockets.
- [ ] Add fixtures for descriptor/ephemeral-port information being available versus permission-denied/unavailable. Runtime unknown values serialize as unavailable, not zero. Capture actual generator limits from its running Go process, not the launching shell.
- [ ] Add an owned PostgreSQL integration test with a one-connection pool: hold its connection, start a waiting acquisition, release it, then assert acquired/idle/total limits and wait counters are reported correctly. Test numeric metric names and units without DSN labels.
- [ ] Implement private consumer pool metrics, driver pool sampling, effective API/Fiber settings, Caddy adapted configuration/default provenance, PostgreSQL session aggregates by test application, and owned process/resource snapshots. Capture the existing serial write behavior; do not add writer parallelism.
- [ ] Classify publish errors into bounded labels `buffer_saturated`, `timeout`, and `broker_error`; record configured/current producer buffered records/bytes using APIs verified in pinned franz-go. Do not infer record-buffer limits from socket counts. Add tests for all three classes and for secrets/event IDs never becoming metric labels.
- [ ] Run package tests/race checks and the targeted owned integration tests. Commit instrumentation without changing production pool sizes, broker cores, buffer limits or delivery policy.

## Task 4: Provide one official measured-run path

**Files:** Modify `cmd/bench/main.go`; add `cmd/bench/main_test.go`, `internal/benchmark/calibration.go`, `calibration_test.go`, `connections_cases.go`; modify `cases.go`, `study.go`, `progress.go` and their tests.

**Interfaces:** Measured single-run modes use `runenv.Start(Profile:"selfhost", ExposeDependencies:true)`, the generated API key/default tenant and `Stack.HTTPClient()`. Record `MaxConnsPerHost` independently of worker count and keep idle retention explicit. Export the existing ownership-scoped progress helper as `WaitConsumerCaughtUp(ctx, stack) error` for official callers. Add `StudyConfig.Suite` with supported values `core` (default) and `connections`; `-suite` applies only to study mode. Add `HTTPMaxConnsPerHost` and `HTTPMaxIdleConnsPerHost` integer fields to `Case` and `PipelineConfig`, captured as separate public control fields. Active cap zero means unlimited; idle zero means the transport default and must be recorded with its effective value. `ConnectionCases() []Case` implements the spec's fixed 12-case matrix. Add `-mode calibrate` for a labelled TLS acknowledgement sink with no broker/database claims.

- [ ] Test that measured modes select the durable self-hosted profile, use the correct TLS client/auth/tenant, reject unknown flags/suites and never print private URLs/keys. Keep the smoke path small.
- [ ] Test the seven cap settings 4, 8, 16, 32, 64, 128, 0 at fixed workers/idle retention 128, three ten-second windows and 1 KiB/100-event batches. Test that the 30k/50k cases use cap 128 and reuse the 15k/cap-128 baseline explicitly; test exactly 12 unique cases overall.
- [ ] Test the separate payload cohort: 256/1,024/16,384-byte payloads, batch 10, rate 1,000, workers/active-cap/idle retention 128, three ten-second windows. Generate an actual encoded 16 KiB request and assert it fits `event.MaxRequestBytes`; reject oversized configurations before launching infrastructure. Keep the API body limit unchanged.
- [ ] Implement calibration against an owned local TLS sink with known matching acknowledgement IDs. Assert request/event accounting and transport instrumentation; do not put flaky throughput assertions in CI. Calibration output is explicitly a driver test, never an ingestion result.
- [ ] Update the shared study runner's namespace isolation, per-sample offset catch-up, cleanup, manifest controls and compressed artifact bounds for the suite. Do not reuse the ignored ad-hoc probes as the published runner.
- [ ] Verify that a 60-second/50k request is refused with a clear duration/count-limit explanation under the existing guard; do not raise the guard or raw artifact limits here. The sustained-run follow-up is a separate milestone.
- [ ] Run CLI/unit/race tests and `make bench-study-smoke`. Commit the official run path.

## Task 5: Run the controlled connection study

**Files:** Create only the new immutable bundle `benchmarks/results/connections-<date>-<run-id>/` after the measured source commit is clean. Store preliminary/incomplete attempts outside the public tree.

- [ ] On the current host, run driver calibration at 150/300/500 requests/s, with 100 events/request and 1 KiB payloads. Record misses, waits, active sockets, CPU/FD limits and timing; a generator-limited case cannot establish a server ceiling.
- [ ] Run the seven-setting connection suite exactly as specified. Record actual connections, driver delays, producer pressure, database acquisitions/waits, server descriptors and storage/offset progress. Keep payload, target rate, workers, idle retention, protocol and server settings fixed across that sweep.
- [ ] Run the predeclared high-rate cohort at cap/workers/idle retention 128, using its shared 15k baseline and new 30k/50k cases. Run the separate three-size, batch-10/rate-1,000 payload cohort. Do not silently select a best connection setting or mix payload cohorts; keep all three repetitions and every overload/failure outcome with metric validity.
- [ ] If a pool/OS/proxy limit is actually reached, add a separately named one-variable comparison. If no wait/error indicates a DB connection constraint, leave DB limits unchanged and state that evidence.
- [ ] Independently audit raw request/event counts, complete ID/payload verification, timing validity, all source/settings hashes, and the real connection statistics. Any invalid metrics stay unavailable; interrupted attempts are archived externally and are not used in numeric comparisons.
- [ ] Accept the bundle only as local screening evidence. If the driver cannot offer a target, label that result generator/connection constrained and report the actual achieved rate. Never substitute the offered target into a headline.
- [ ] Verify every owned stack is removed and no credentials appear in the public capture. Commit the immutable audited replacement bundle; retain valid overload results.

## Task 6: Publish concise, accurate reader material

**Files:** Modify `README.md`, `benchmarks/README.md`, `docs/{article,architecture,operations,integration,development}.md`, `CONTRIBUTING.md`, `docs/figures/README.md`; create `docs/figures/manifest.json` and `benchmarks/plot_connections.py`; remove retired plot-generator/receipt files only after references are migrated. New measured figures, if supported: `connection-pressure{,-mobile}.svg` and `acceptance-storage{,-mobile}.svg`.

- [ ] Explain request concurrency, TCP connections, idle pooling, record buffering and DB pooling with the measured values and one short example. Use Humanizer; preserve factual qualifications and working commands.
- [ ] Replace retired links and unsupported 25-case/COPY/capacity statements with the current audited scope. Keep deployment and retry instructions; separate valid database-method comparisons from any unmeasured production optimization claim.
- [ ] Plot only independently audited schema 2 metrics. One figure answers how active connection caps change achieved load/waits; the other separates acceptance from stored/offset progress over time. Show actual units, all repetitions and failed/missing metric gaps. Do not present observed-pending counts as exact queue size.
- [ ] Replace old receipts with a fresh manifest listing retained source-backed assets and new measured outputs, exact source/data hashes and generators. Recheck the retained diagram JSON's source ranges against the final code; distinguish original export provenance from current source validation. Do not edit an old receipt to imply an old figure used new data.
- [ ] Verify every local documentation/image link, regeneration command and figure label. Preview desktop and phone layouts; do not create decorative images or repeat the entire topology in every page.
- [ ] Commit reader updates. No sustained capacity headline or full article rewrite is part of this cleanup milestone.

## Task 7: Remove leftover clutter and complete review

- [ ] Archive the old seven workflow documents and confirmed inactive ignored run/probe/preview directories using Task 1. Extract relevant operational checks first; keep example source, outbox data, active worktrees, current plan/spec and unrelated projects intact.
- [ ] Finish self-review against every spec requirement and run the repository's full Go race/vet/build, language-client/outbox checks, independent study auditor, full owned Docker integration suite and demo cleanup checks once for the final implementation. Use the existing project commands, not invented test shortcuts.
- [ ] Obtain the required final whole-branch independent review before the existing PR/merge workflow. Resolve each material finding with evidence.
- [ ] Archive the completed current plan/spec and final review notes externally if they would otherwise add redundant public workflow documentation. Retain their Git history and archive mapping; do not invalidate a review by removing runtime/test files afterward.
- [ ] Verify `git ls-files` contains none of the three retired study roots, seven retired measured SVGs, obsolete receipts/plot script or old workflow documents; confirm all public links resolve and all remaining reported measurements have current validity/provenance. Summarize exactly what was removed, retained and replaced.
- [ ] Create/attach the PR; merge only after required checks are green, using the user's existing integration preference. Preserve source revisions needed to reproduce the new measured study.

## Separate follow-up: sustained capacity and the article

A dedicated deployment and separate load generator are not currently provisioned.
That does not block Tasks 1–7. Before a sustained 30k/50k claim, design bounded or
streaming generation/verification and segmented immutable captures for at least
three million events per 60-second run; preserve a continuous load clock and
artifact safety bounds. Then predeclare storage/offset-backlog stability and
achieved-rate criteria, repeat independently, and plan the article around the
validated result. This follow-up must not be replaced by a larger worker flag,
disabled durability, deleted overload runs or a renamed offered-rate column.
