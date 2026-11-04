# Self-hosted IngestKit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the collector straightforward to self-host and integrate, with reproducible experiments and clear visual documentation.

**Architecture:** A Caddy HTTPS gateway fronts the existing Go API, with Redpanda and PostgreSQL private inside a single-host Compose deployment. Application examples use plain HTTP and a durable outbox. An owned local study runner measures that deployment with independent event reconciliation and publishes figures from its captures.

**Tech Stack:** Existing Go/Fiber/franz-go/pgx runtime and pinned database/broker/build images; verified pinned Caddy image; POSIX shell/OpenSSL for local configuration; Python/Node standard libraries for integration; matplotlib and Archify for development-time figure generation.

**Spec:** docs/superpowers/specs/2026-11-03-self-hosted-ingestion-design.md (approved 4 November 2026; native execution already selected).

**Status:** Native implementation Tasks1–10 and the fresh whole-branch review/fix pass are complete in the existing feat/self-hosted-ingestion worktree. Full post-fix integration passed all 31 checks in 570.789s. The user subsequently selected push, pull request and merge after checks; integration is tracked by the pull request and Git history. See the [verification and review record](../reviews/2026-11-04-self-hosted-ingestion.md). The fsync correction and historical-results erratum are already committed in 3ca1a41, followed by reviewed design 641fbe2. User's go-ahead authorizes this implementation without another approval cycle for a transcription of the same scope.

## Global Constraints

- Preserve Go runtime, existing HTTP envelope/response contract, stable client IDs, tenant scoping and commit-before-offset ordering. HTTP limits remain256KiB/event,1MiB/request,100events/batch.
- A202 means broker acknowledgement, never database persistence. Preserve ambiguous retries with identical IDs and bytes; first stored content wins.
- Broker process fsync bypass must be false, topic write caching false, PostgreSQL fsync/synchronous_commit enabled. Preserve historical raw captures with their correction; new measurements require committed clean code and actual settings.
- Chosen target: one self-contained Linux/Compose server. Only gateway80/443 public; no public database, Kafka/admin or metrics ports. No HA, remote provisioning, paid resources, DNS changes, registry releases or social/article posting.
- Generated secrets are ignored mode0600 files, not routine log output, source files or build context. Never overwrite existing configuration. Consumer database credentials have only required privileges.
- Every test/demo/study owns unique projects, volumes and ports and verifies ownership before control/cleanup. Preserve unrelated data and ordinary deployment volumes. No global prune, destructive operator restore/reset, or history rewrites.
- New study has exactly11DB,11pipeline and3recovery configurations; at least3 measured repetitions after warm-up. Pipeline large-payload cases use batch10 and its256B/batch10 control.
- Local TLS tests trust an explicit private CA; never disable certificate checks or claim public ACME was tested without a public host. Deployment host does not need Go.
- Keep implementation native, using small tested commits and a persistent task ledger. One fresh whole-branch review at completion; no implementer/reviewer per task.

## Review Focus

1. A shell hostname or existing output path must not inject commands, corrupt env files or overwrite secrets; Docker context/logging must not expose nested credentials. Task1/2 tests.
2. API acceptance must continue with the restricted writer role, but denied writes must make readiness false even when Ping succeeds; restart/grant/credential updates preserve stored IDs. Task2/3 tests.
3. A sender may die after HTTP acknowledgement before marking an outbox row; restart must reuse the exact event ID/body without duplicating storage, while permanent rejection remains inspectable. Task4/5 tests.
4. Observation cannot advance a timestamp cursor past a transaction that commits late; high load, late ACKs, missing rows and client misses must not become a false successful capacity claim. Task6/7 tests.
5. Interrupted/resumed studies and backup restores cannot act on another stack or mix revisions/controls; retain failure evidence and verify every configuration/repetition exists. Task3/7/8 tests.

## File boundaries

- deploy/: canonical deployment Compose, Caddy and database-role setup, config generation and operator guide. Root development Compose remains supported.
- internal/runenv/: owned deployment-profile support, trusted local HTTP client, explicit test-only loopback dependency bindings; existing ownership checks remain authoritative.
- internal/service/: safe bounded diagnostics; cmd/consumer only wires them alongside processing health.
- examples/checkout/: importable Python/Node HTTP helpers and thin current CLIs; examples/outbox/: one SQLite order/outbox application.
- internal/benchmark/: deterministic profiles/namespace, incremental observer, valid rate accounting,25-case registry, journal, compressed artifacts and recovery orchestration.
- cmd/bench/: existing single-run modes plus study CLI; docs/assets + figure sources: derived outputs; docs/: short reader paths and article.

### Task 1: Safe first-use deployment configuration

**Files:** Create deploy/configure.sh, deploy/.env.example, deploy/README.md, internal/runenv/deployment_config.go and deployment_config_test.go; update .gitignore/.dockerignore if needed.

**Interfaces:** deploy/configure.sh HOSTNAME [OUTPUT_DIRECTORY] defaults output to deploy/.env; it requires only shell/OpenSSL. Produce DeploymentConfig{Domain string, APIKeys map[string]string, AdminPassword, WriterPassword string} and ReadDeploymentConfig(path string)(DeploymentConfig,error). Generated passwords/keys are64hex characters; initial tenant default. Multiple valid key mappings permit overlap rotation. Parse data, never source/eval env files.

- [x] **1. RED tests:** TestConfigureCreatesPrivateUniqueSecrets, TestConfigureRejectsUnsafeHostname, TestConfigureNeverOverwrites, TestDeploymentConfigRejectsMissingOrDuplicateFields. Run the real script against temp directories. Assert0600 mode, fresh64hex values, no stdout/stderr secrets, exact localhost/events.example.com domain acceptance; reject schemes, ports, whitespace, shell substitutions and newline inputs. Failed generation leaves no misleading partial config; existing sentinel bytes remain unchanged. Config round-trip verifies a valid mapping; empty fields, malformed JSON, invalid tenant text and duplicate assignments fail.
- [x] **2. Run:** go test ./internal/runenv -run 'TestConfigure|TestDeploymentConfig'. Expected: missing implementation or incorrect script behavior, not a permission/config skip.
- [x] **3. Implement** only generation/parsing, safe exclusive file creation and ignored credential paths. Secrets keys: DOMAIN, API_KEYS, POSTGRES_ADMIN_PASSWORD, POSTGRES_WRITER_PASSWORD; optional retained controls get explicit defaults later. Add recursive env/private-data exclusions to Docker context without excluding required Go sources.
- [x] **4. Verify:** go test -race ./internal/runenv and go test -race ./...; expected all pass. Inspect generated non-secret field names, never dump credentials. Document configuration command only; full deployment commands arrive with Task2.
- [x] **5. Commit:** feat: generate safe self-hosted deployment configuration.

### Task 2: Actual HTTPS deployment, private services and restricted writer

**Files:** Create deploy/compose.yml, deploy/Caddyfile, deploy/roles.sql; extend internal/runenv/compose.go, create deployment.go and deployment integration checks under tests/integration/deployment_test.go; add owned override fixtures under tests/fixtures/deployment/; update deploy/README.md and CI test invocation/time budget as required.

**Interfaces:** Extend runenv.Options with Profile string (empty/development or selfhost) and ExposeDependencies bool (selfhost test/study only). Stack records private profile/config/CA; add APIKey() string and HTTPClient()(*http.Client,error). Selfhost APIURL ishttps://localhost:<ownedport> in tests, trusting an extracted private CA. ExposeDependencies adds explicit loopback-only PostgreSQL/Kafka/consumer probes for tests/studies and sets existing DatabaseURL/Brokers/ConsumerURL fields; canonical deployment has none of those published ports. Root development behavior stays unchanged.

- [x] **1. RED real checks:** TestSelfhostTLSFirstEventAndDuplicate, TestSelfhostPublishesOnlyGateway, TestSelfhostWriterPrivileges, TestSelfhostRestartPreservesIDs, TestSelfhostMissingConfigFails. Verify a valid TLS chain through gateway, default client rejection without its CA, authenticated single/batch delivery, duplicate row count1,404 on public metrics/admin routes and only80/443 bound by canonical services. Writer role is non-superuser, writes through the actual store and cannot create/drop permanent tables or act as admin. Data and migration ledger survive restart. Missing secrets/domain fail startup with field names, no secret echo. Existing forged-owner and startup cancellation checks continue passing.
- [x] **2. Run** scripts/test-integration.sh with the new tests selected; expected fail before deployment exists. Check pinned Caddy/broker CLI and image provenance against official documentation before adding exact versions/options.
- [x] **3. Implement** gateway + PostgreSQL admin/migration + idempotent restricted role/grants + existing API/consumer + broker/topics. No dev-container implicit defaults in canonical deployment; explicitly set safe fsync behavior, appropriate private listeners and documented container resource settings. Grant writer TEMP, schema USAGE and required event table privileges; migrations retain their separate ledger. Configure reload recreates services, not only restart. Add local-CA override only to owned tests, and dependency probe bindings only when ExposeDependencies=true. Keep generated env/CA files private; collect safe stack evidence and preserve failure cleanup.
- [x] **4. Verify** complete real integration suite, unit/race, vet/build, real rendered Compose config and image-build context exclusion using a nested credential canary. No manual source-text assertion replaces actual published-port/role/TLS checks. Document clone→configure→up→send→SQL verification→volume-preserving down and the public-domain80/443 prerequisite.
- [x] **5. Commit:** feat: deploy the collector behind HTTPS with private dependencies.

### Task 3: Operator diagnostics, credential refresh and verified restore

**Files:** Create internal/service/diagnostics.go/diagnostics_test.go; wire cmd/consumer/main.go; extend tests/integration/deployment_test.go; create docs/operations.md; finish deployment operator links.

**Interfaces:** NewDiagnostics(logger *slog.Logger,cooldown time.Duration)*Diagnostics; Report(kind,topic string,partition int32,offset int64,err error). Known kinds only; emit safe error class/SQLSTATE and source coordinates, suppress repeats for10s with a count. No raw err.Error, keys, payloads or DSNs. Use existing messaging.Operation callback and processing health. Startup failures have explicit unavailable coordinates, not invented offsets.

- [x] **1. RED:** TestDiagnosticsRedactsAndBoundsRepeatedFailures, TestWriteDeniedMakesDeploymentUnready, TestCredentialOverlapAndRefresh, TestBackupRestoresIntoOwnedReplacement. Controlled clock verifies initial failure, suppressed repeats, recovery and later failure. Deliberate error text contains secret/payload sentinels that must not appear in application logs. Revoke INSERT from actual restricted writer while Ping works: readiness503, accepted IDs pending; restore grant and verify successful persistence/readiness. Recreate API with overlapping old/new keys for the same tenant, then remove old key: existing data survives, old401/new202. Restore a PostgreSQL custom-format dump into a separate owned empty replacement, verify original IDs/content and subsequent new/duplicate ingestion; source stack remains untouched.
- [x] **2. Run** affected unit/real tests and observe failure before wiring diagnostics/recipes.
- [x] **3. Implement** safe classification/throttling and exact operator commands: status, safe logs, pending events, key overlap, retention/disk growth, upgrades, backup and fresh-target restore. Explain explicit replay from retained broker data when restoring an older DB with advanced offsets; never automate destructive live offset resets/schema rollback. Tests may replace only their own verified empty fixture database/schema. Secrets are not printed in command diagnostics.
- [x] **4. Verify** targeted tests plus full integration and race suite. Exercise the written backup/restore and configuration-recreate commands on owned resources. Preserve existing supervisor/drain semantics.
- [x] **5. Commit:** feat: expose safe operation diagnostics and verified recovery procedures.

### Task 4: Importable application HTTP clients and API guide

**Files:** Create examples/checkout/ingest_client.py, ingest-client.mjs and standard-library tests; refactor send.py/send.mjs; add minimal Python package markers as needed; create docs/integration.md and docs/api.md; update examples/README.md, checkout README and CI.

**Interfaces:** Python Client(base_url,key,timeout=8,max_attempts=3,ca_file=None) with send(event) and send_batch(events); Node IngestClient({url,key,timeoutMs=8000,maxAttempts=3,caFile}) with send/sendBatch. Return validated acknowledgement IDs; typed/sanitized errors expose status/retryable/request ID. Node uses standard http/https to support an explicit test CA. CLIs retain API_URL, INGESTKIT_API_KEY, SEND_MODE, EVENTS_FILE and add INGESTKIT_CA_FILE.

- [x] **1. RED controlled-server tests:** import has no side effects; single/batch return exact IDs;503/429/network ambiguity preserve serialized IDs/body and stop after bounded attempts;400/401/413 do not retry; mismatched ACKs cannot become success; key/DSN/payload are absent from error output; untrusted TLS fails and supplied CA succeeds. Retain existing real TestCallerModesPreserveRetryBytes and run both clients through owned HTTPS deployment.
- [x] **2. Run** python3 -m unittest discover -s examples/checkout -p 'test_*.py' and node --test examples/checkout/*.test.mjs; expected new behavior fails.
- [x] **3. Implement** small copyable modules, no generated SDK or package publishing. Caller supplies stable IDs and trusted backend credentials. Guide covers real backend usage, broker acceptance vs persistence, atomicity limits of sending after a business write, all status codes/limits and SQL verification. Browser secrets stay on its backend.
- [x] **4. Verify** both language suites, real integration contract checks, Go race suite and documented import/CLI commands. CI runs language suites after installing its existing Node version.
- [x] **5. Commit:** feat: provide reusable Python and Node ingestion clients.

### Task 5: Durable application outbox example

**Files:** Create examples/outbox/{app.py,__main__.py,__init__.py,schema.sql,test_outbox.py,README.md}; add tests/integration/outbox_test.go and test-only crash wrapper; extend integration guide.

**Interfaces:** create_order(database,order_id,amount_cents)->event_id; dispatch_once(database,client,batch_size=100)->summary. CLI: python3 -m examples.outbox create-order|dispatch|status with --database; dispatch is a bounded one-shot command. One SQLite transaction stores order, UUID, timestamp and serialized event. Retried same order ID returns its original event; conflicting amount fails. Pending/acknowledged/rejected states are explicit; acknowledged always means broker ACK.

- [x] **1. RED:** atomic order/outbox rollback, stable ID on retried business request, unchanged bytes after process restart, bounded pending retry, visible permanent rejection and no false acknowledgement. Real TestOutboxSurvivesBrokerOutage and TestOutboxCrashAfterAckReplaysSafely: crash wrapper pauses only the test dispatcher after real HTTP ACK before SQLite update; kill own child, restart, verify one stored UUID and eventual outbox ACK. No production HTTP/test failpoint.
- [x] **2. Run** standard Python outbox suite and affected real integration tests; expected fail before app exists.
- [x] **3. Implement** using Python sqlite3 and Task4 client. Retain retryable/ambiguous work, expose permanent rejection for operator repair, no silent dropping. Explain single-dispatcher scope and how to adapt the transaction to an application's own DB; include final SQL/outbox status commands. Do not build an analytics UI or queue framework.
- [x] **4. Verify** Python/client suites, Go race and full real integration; run documented order→dispatch→collector SQL workflow over HTTPS with the private CA.
- [x] **5. Commit:** feat: demonstrate durable application delivery with a transactional outbox.

### Task 6: Measurement integrity before scaling experiments

**Files:** Extend internal/benchmark/workload.go, database.go, pipeline.go/results.go/manifest.go and tests; create observer.go/observer_integration_test.go; update benchmark methodology.

**Interfaces:** WorkloadConfig gains Namespace and PayloadProfile (legacy/fixed versus varied). Namespace scopes reproducible UUIDs and benchmark event type; varied payloads are deterministic ASCII entropy at the exact requested encoded size. PipelineConfig gains HTTPClient *http.Client and Namespace. PendingObserver uses one pool connection and a temporary pending-ID relation; Observe(ctx)([]StoredObservation,error), Close(ctx)error. It removes IDs only when a query sees their committed target rows. Keep a final independent content/set query. Add accepted-in-window vs late-ACK counts, query/poll overhead and explicit observation-bounded ambiguity fields.

- [x] **1. RED:** seed/profile/namespace reproducibility and exact byte sizes; duplicate bytes preserved; invalid counts/NaN fractions rejected; different cases cannot reuse IDs. TestObserverSeesOutOfOrderCommits holds an earlier-received/insertion transaction open, observes a later transaction, then commits the first and must still see it. TestObserverCancellationReleasesConnection, TestIndependentReconciliationFindsMissingAndUnexpected, TestLateAckOutsideMeasuredWindow, TestClientQueueTailClassified, and secrets-redaction checks. Preserve unowned sentinel rows and refuse pre-existing case collisions before cleanup.
- [x] **2. Run** unit and tagged DB tests; verify failures identify the existing full-scan/cursor/accounting limitations.
- [x] **3. Implement** incremental temp-table observation and actual-source metadata, including current broker memory/CPU settings, deployment route, selected safe API controls and TLS use. Never use received_at/persisted_at as a commit cursor. Final verification checks expected IDs/content and unplanned IDs within the run's unique event-type namespace, not unrelated rows. Observational latency remains an upper bound. Acknowledged missing events fail; generator misses/rejections/ambiguous attempts are explicit limits, never a successful attained-load/capacity headline. Missing ambiguity observations are unknown-by-deadline, not proof of non-delivery.
- [x] **4. Verify** full Go race/vet/build and real observer/pipeline correctness checks. Capture no credentials when inspecting actual settings. Maintain historical report readers; do not edit historical raw captures.
- [x] **5. Commit:** fix: make expanded ingestion measurements reproducible and commit-aware.

### Task 7: Owned25-case study runner, journal and recovery experiments

**Files:** Create internal/benchmark/{cases,study,recovery,artifacts}.go and tests; extend cmd/bench/main.go and Makefile; add small integration smoke and benchmark guide.

**Interfaces:** StudyCases()[]Case is the exact25-case registry from the spec; Case has ID, Kind and explicit controls. StudyConfig selects output directory, full/smoke, resume and seed. RunStudy(ctx,config)error owns a selfhost stack with test-only loopback observers and trusted HTTPS client. Existing single-run benchmark modes remain. Add make bench-study and make bench-study-smoke. Journal schema1 stores source SHA, image/config fingerprints, seed, run namespace, case/repetition progress and artifact hashes. Resume reruns incomplete work on a fresh owned stack and refuses differing source/config; completed captures stay immutable.

- [x] **1. RED:** TestStudyRegistryHas25ControlledCases checks11DB/11pipeline/3recovery and matched large-payload batch10 baseline; serialized requests remain below1MiB. TestJournalRejectsRevisionOrControlChanges, TestStudyInterruptPreservesEvidenceAndOwnership, TestEveryMeasuredRepetitionRetained, TestCompressedArtifactsHaveStableChecksums. Real smoke covers one small DB and one HTTPS pipeline case,3reps each. Recovery tests record actual fault/restart timestamps and accepted/stored sets rather than synthetic curves.
- [x] **2. Run** targeted tests; expected missing registry/journal/recovery behavior, no silent service skip.
- [x] **3. Implement** sequential progress reporting and atomic journal writes. Full DB count5000,3reps, rotated methods; pipeline controls exactly spec (baseline10s,2s warm-up,30s drain,50ms target polling); CI smoke uses small counts/windows but3reps. Recovery: DB down at3s/up at8s of15s500/s; consumer kill at3s/restart at5s in same workload; outbox100events with broker down8s then restored. Clean with fresh bounded context, restoring stopped services before collecting evidence/destroying only owned resources. Public summaries JSON/CSV remain readable; large samples use gzip mtime0 and SHA256. Retain limited/inconclusive cases with reasons; correctness failures exit nonzero. No external DB/host flags in study CLI.
- [x] **4. Verify** registry/journal/artifact unit tests, race suite, full integration including new study smoke and all three recovery cases. Document full versus smoke commands and resume behavior. Commit the entire harness before measurements.
- [x] **5. Commit:** feat: run reproducible self-hosted ingestion studies and recovery experiments.

### Task 8: Execute and audit corrected experimental evidence

**Files:** Add bounded captures under benchmarks/results/selfhost-2026-11-04/ with index, manifests, raw artifacts, checksums and methodology; update results links only after validation.

**Interfaces:** Execute the committed full25-case study sequentially; preserve its clean source SHA and every3-repetition outcome. Figure inputs are the study index plus case summaries, not a hardcoded old capture directory.

- [x] **1. Run** an untimed feasibility/smoke check of the committed harness, then the full study with progress redirected to a readable log and bounded tool waits. Do not run unrelated load/tests concurrently. If a concrete environment limit needs changed timing, ledger it and apply it consistently to matched controls; never silently drop a case.
- [x] **2. Audit** all25 cases,3reps, correct method counts, timing populations, exact ID/content outcomes, actual fsync/settings/HTTPS route and no secret material. Recompute summaries/percentiles/checksums independently. Show target-vs-generated load and limits. Keep failed cases and investigate genuine correctness failures rather than deleting them.
- [x] **3. Evaluate** the DB crossover honestly. Do not advertise COPY as faster where it loses. Default-writer changes require a concrete separately recorded decision, equivalent failure tests, and rerunning affected pipeline captures from the resulting committed code; no guessed adaptive threshold.
- [x] **4. Verify** study artifacts against raw samples and manifest schema; integration/measurement code remains tested. No headline capacity, cost, public-TLS issuance or production-adoption claims. Old development-mode captures remain historical.
- [x] **5. Commit:** bench: record corrected self-hosted ingestion experiments.

### Task 9: Readable code-backed diagrams and evidence figures

**Files:** Add figure sources under docs/figures/, generated deployment.svg, delivery.svg, database-batching.svg, pipeline-load.svg and recovery.svg under docs/assets/; refactor benchmarks/plot_results.py to consume the study index and write reproducible SVG/PNG. Keep Archify candidate/HTML evidence in ignored artifact folders unless a small source is needed for reproduction.

**Interfaces:** Scientific plots derive all values from Task8 captures; diagrams cite committed source using the inspected revision. Archify produces checked architecture/sequence artifacts and static SVG exports; no imagegen for data charts or code-native diagrams.

- [x] **1. Verify inputs** and choose one coherent typography/color system. Deployment figure shows actual trust/port boundaries; sequence distinguishes broker ACK, DB commit, offset and replay; charts answer the batch/load/recovery questions with controls, units and all repetition variation.
- [x] **2. Generate** using matplotlib and the Archify skill's native authoring/export workflow. No invented data, smoothing that implies unmeasured points, decorative services or claims from invalid cases. Caption every omitted/limited population. Include standalone readable provenance/caveats where image reuse would otherwise mislead.
- [x] **3. Inspect** images at normal README width, a narrow viewport and light/dark backgrounds; inspect rendered Markdown too. Text remains readable with no cropped legends/arrows/labels. Repair visuals, not the underlying measurements. Keep exact data/control links beside each figure.
- [x] **4. Verify** deterministic regeneration, SVG validity and relevant artifact/browser gates. Editorial/generated-asset work earns visual/data checks rather than source-text unit tests.
- [x] **5. Commit:** docs: illustrate deployment integration and ingestion tradeoffs clearly.

### Task 10: Reader documentation, article and final branch review

**Files:** Rewrite README.md, docs/{architecture,delivery,development,integration,api,operations,article}.md, deploy/README.md, examples docs, benchmarks guide and publication/resume drafts; update plan/spec verification notes.

**Interfaces:** README leads deploy→send→verify→operate, with concise limitations and links. Article connects deployable use, application/outbox integration and actual measured tradeoffs; all numbers point to clean revisions/captures. No publishing performed.

- [x] **1. Inventory** every public command, local link and numeric/delivery claim against existing executable tests/source/results. Consolidate repeated setup; keep planning records off onboarding. Explain operator prerequisites and actual measured resource observations without inventing host sizing guarantees.
- [x] **2. Rewrite** the article and integration/operations narrative from final evidence. Show practical configuration and imports, screenshots/figures only when actually captured, and why each recovery result matters. Publication/HN/resume material remains a local draft with no adoption or popularity claims.
- [x] **3. Exercise** fresh owned deployment startup, trusted TLS, first event/SQL, clients/outbox, key refresh, backup/restore and stops using the documented commands. Check maintained Markdown links and rendered figures/article. Run Go race/vet/all builds, Python/Node tests, full integration, study-smoke validation, artifact audit and git diff --check.
- [x] **4. Commit** docs: document self-hosted ingestion and measured operating behavior. Complete task ledger, then request the one fresh whole-branch review required by native execution with the spec/plan/rulings and Review Focus above. Re-grade findings; fix Important/Critical in one RED→GREEN pass and rerun the affected and full regression gates. Record deferred minors/rulings; no re-review.
- [ ] **5. Finish** through the branch-finishing skill with actual evidence. Preserve user data and this checkout; do not infer authorization for real infrastructure or publication. Remove only this plan's completed scratch workspace after retaining its decisions/results for final reporting.

## Plan self-review

One dependency chain rather than independent projects: safe config→deployment→operator/integration→outbox→valid HTTPS measurement→25-case evidence→figures/article. Existing fsync correction already meets the prerequisite. Shared interfaces are named above. All five Review Focus classes have specific real behavioral tests. Reader docs and CI accompany their functional tasks; final writing uses only performed measurements. No important approved requirement is intentionally omitted.
