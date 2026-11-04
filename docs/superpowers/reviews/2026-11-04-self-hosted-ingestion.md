# Self-hosted ingestion: verification and review

Implemented on `feat/self-hosted-ingestion` from base `698d7b8`.

## Verification

- Documentation/runtime head before review: `9252894`.
- Go race suite, vet and builds passed.
- Python client 11, Node client 7, outbox 5 and independent auditor 6 tests passed.
- Before-review real Docker integration: 31 top-level checks passed in 585.069 seconds, including trusted HTTPS, private ports, writer privileges, denied-write readiness, restart preservation, credential refresh, fresh-target backup restore, caller modes, outbox after-ACK crash, study smoke/resume/interruption and three recovery scenarios with three repetitions.
- Both complete 25-case studies independently audited: each 99 database rows, 33 pipeline windows, 9 recovery repetitions; all 9 limited cases retained.
- Fresh documented deploy→send→SQL→client→outbox→volume-preserving stop walkthrough passed against an owned trusted-CA deployment. Public ACME/DNS was not exercised.
- 20 maintained Markdown files / 146 local links and images checked; actual desktop/mobile article and figures inspected. Chart exports regenerate byte-identically; asset hashes in `docs/figures/verification.json`.

## Fresh review and fix

A read-only fresh-context review used gpt-6-astra with high reasoning effort over `698d7b8..9252894`, after all ten task completion gates. It inspected the runtime, harness, deployment/operations, clients/outbox, studies, figure sources and every ruling, independently reran both study audits and inspected the full integration evidence. No competing Docker or full timing study was run.

The reviewer found no Critical issue and one Important issue: Python DNS resolution and sequential address/TLS setup could exceed the claimed per-attempt timeout. Its delayed DNS probe took 0.31s with a 0.03s configured timeout.

Accepted and fixed in `802ce89`: a monotonic attempt deadline spans DNS, all address connection attempts, TLS and HTTP. At most eight DNS-only daemon workers may outlive a timeout; they hold no event/key data and cannot initiate a connection or POST. Socket failures close the failed connection, TLS keeps original-host verification/SNI, retries retain the original serialized body.

Three regressions failed first: delayed DNS 0.319s, repeated address timeouts 0.192s and blocked DNS work 2.515s. All 11 Python tests then passed, along with Node 7/outbox 5/auditor 6 and Go race/vet/build. The full post-fix Docker integration passed all 31 checks in **570.789 seconds** on `802ce89`, including all three recovery scenarios with three repetitions. No re-review was dispatched, following the approved native one-fix-pass workflow.

## Rulings I made

1. Treat the user's go-ahead as approval to implement the already reviewed design and its concrete native plan — asking again would repeat an approval already supplied — detailed implementation choices receive regression tests and the final fresh review instead of another planning gate. Cost if wrong: reviewable implementation could exceed the intended planning scope; no public side effect is authorized.

2. Reuse the existing ignored Git worktree — the app's native creation tool previously returned Not a git repository for this clone, so the established Git fallback is already isolated — the app does not manage this worktree; use its explicit path and preserve it for review. Cost if wrong: app lifecycle integration/bookkeeping is missing; the primary main checkout remains untouched.

3. Pull the observer-clock correction forward from benchmark validity work — the full gate found negative host-minus-API timestamps, and an owned 20-query probe proved the deployment VM was about53.7ms ahead of host after each SQL response. Receipt and SQL observation now use the same deployment VM clock, while host monotonic timing retains scheduling/backlog duties. Query clock_timestamp after reading committed rows, include probe delay in the documented upper bound, and carry StoredObservation.ObservedAt into Task6's observer. Cost if wrong: same-clock assumptions would still be invalid with API/database on different hosts; the supported study/deployment is one host, and negatives still fail rather than clamp. Raw historical captures remain unchanged.

4. Include the final independent ID/content query and observer disposal in the reported observation drain/total duration — the final census can discover a late ambiguous row, so using that row with an earlier denominator would overstate storage rate. Record VerificationNS and ObservedEvents separately; latency quantiles include only actual pending-observer samples. Cost if wrong: rates conservatively include benchmark verification overhead and are not intrinsic service capacity; captions and methodology state this, and old raw captures remain unchanged.

5. During the deliberate PostgreSQL outage only, rebuild the observer's connection-owned pending relation from IDs not already observed after reconnecting — stopping PostgreSQL also destroys the observer session, so treating that expected gap as permanent would prevent the availability experiment from measuring recovery. Record unavailable observations and limits; retain strict final independent ID/content verification. Cost if wrong: visibility during the gap cannot establish a precise commit time, and the availability report must show the gap rather than imply continuous observation or capacity.

6. Compare canonical image content layers plus full runtime configuration while excluding only the proven per-project Compose label; retain exact opaque image IDs in each immutable manifest — fresh owned projects necessarily receive different labels, so equality of opaque IDs alone cannot support resume. Cost if wrong: software that interprets that label as behavior would require it as a control; these runtime services do not read Docker labels, and all other configuration/content remains fingerprinted.

7. Switch the deployed EventStore to one atomic multi-row INSERT with ON CONFLICT while preserving the explicit COPY/staging reference in the benchmark — batch INSERT had the lower median elapsed time in all11 corrected DB configurations, so retaining COPY as an assumed optimization would contradict the measured evidence and require unnecessary TEMP privileges. Preserve the entire first25-case study, test atomic failure/replay/restricted-role behavior, and run a new complete25-case study from clean final code. Cost if wrong: unmeasured hosts/workloads may favor COPY; this result is local, the comparison remains runnable, and no adaptive/universal cutoff is claimed.

8. Keep the fully checked Archify sequence for desktop/detail and add small source-cited typographic SVG reading guides for narrow screens — the full five-participant export shrinks text at390px, and Archify's trial one-column workflow still exports786px wide. The guides are labelled companions, preserves ACK/SQL/offset distinctions and explicitly states concurrent HTTP/consumer timing; it is not presented as a second exact packet trace. Cost if wrong: the simplified guide can omit detailed interactions, so the full sequence/source remains linked and all facts cite the committed runtime. The compact deployment guide preserves the same trust and port boundaries. Main deployment/sequence continue through native Archify export; no data plots are hand-drawn.

9. Apply the same decoded-duplicate-key rejection to server startup and the local deployment reader via a shared parser — source inspection after the walkthrough found that standard JSON map unmarshalling silently selected the last tenant for duplicate API keys, while tooling rejected them. Existing valid and overlapping maps retain their behavior; duplicate/escaped duplicate tests failed first, then pass, with credential-safe errors. Cost if wrong: an operator relying on ambiguous last-value-wins configuration must correct it before startup. Published measurements remain pinned to213e4cc's valid generated configuration; this invalid-config hardening does not retroactively remeasure them.

10. HA, host/disk loss and beyond-retention recovery remain outside supported guarantees — one host/replica and finite retention are explicit in deployment/delivery/article, so these are stated limits rather than hidden recovery promises. Cost if wrong: operators expecting redundancy could lose events after the documented failure boundary.

11. Multiple concurrent outbox dispatchers require an application-specific claim/lease protocol — this reference explicitly supports one dispatcher and does not promise coordination. Cost if wrong: parallel dispatch can send redundant requests or race local status updates.

12. Public DNS/ACME issuance and external deployment remain unperformed — trusted private-CA tests exercise the actual route without claiming external reachability or authorizing infrastructure changes. Cost if wrong: an operator still has to diagnose real DNS/firewall/certificate setup.

13. Cross-host receipt/observation timing remains unsupported — API and database clocks share the measured deployment VM; captions describe observed upper bounds and make no distributed-clock claim. Cost if wrong: copying the benchmark to multiple hosts without clock analysis can invalidate visibility durations.

14. Uncatchable process/host kills can require manual owned-project inspection and cleanup — graceful interruption is tested, while SIGKILL/power loss cannot run a cleanup handler. Cost if wrong: orphan test resources may keep consuming disk or memory until inspected.

15. Universal performance/isolated-host capacity remains unclaimed — both studies retain actual source/runtime controls, repetitions, shared-host caveats, limited generation and unknown IDs. Cost if wrong: a reader extrapolating these local samples could choose inadequate resources.

16. Keep the native-generated SVG export bytes and their provenance/hash checks intact — normalization after native export changes the checked artifact without improving diagram behavior; the cosmetic whitespace observation is separately recorded as deferred Minor. Cost if wrong: a whole-branch whitespace style gate remains noisy for these generated files.

## Deferred minors

- Native deployment.svg/delivery.svg include exporter-generated trailing whitespace; rendered output and deterministic native export checks pass, but git diff --check 698d7b8..HEAD reports those historical added lines.

## Evidence boundaries and integration state

The two measured studies remain byte-identical at their cited source revisions. Later configuration validation and Python deadline hardening have regression evidence; they are not new performance captures. No maximum-capacity, replication or public-ACME verification claim is added.

The final whole-branch whitespace check reports 83 native-export whitespace lines in `docs/assets/deployment.svg` and `docs/assets/delivery.svg`; ordinary working-tree whitespace checks pass. This is the sole deferred Minor.

Implementation was verified and committed on `feat/self-hosted-ingestion` before any push or merge. The user subsequently authorized pushing the branch, opening a pull request and merging after checks. Integration is tracked by the pull request and Git history. Public deployment and article publication remain separate. The existing worktree and private owned-run evidence were preserved; only this plan's completed temporary Superpowers workspace was removed after retaining this record.
