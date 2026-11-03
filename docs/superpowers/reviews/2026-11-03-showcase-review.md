# Showcase execution and final review

Completed the approved 13-task plan on refactor/engineering-showcase using native
execution. The public entry points are the root README, delivery/architecture/
development guides, the checkout example, benchmarks and the article. Planning
records are not part of public onboarding. Existing external releases, legacy
database tables and volumes were preserved.

A fresh gpt-6-astra reviewer inspected 4f08e29..a33b90a. It found no Critical
issues, four Important issues and one Minor issue. It also independently
regenerated all six pipeline ID hashes, recomputed latency percentiles and
checked the database medians against the article; all matched the saved data.

## Important findings addressed in one fix pass

| Finding | Change | Regression evidence |
| --- | --- | --- |
| JSON accepted by the API could permanently fail JSONB storage | Reject unrepresentable Unicode/NUL, numeric ranges and excessive nesting before publishing and during broker decoding | TestRejectsUnstoreableJSONAndText, TestIncompatiblePayloadRejectedBeforePublish, TestUnstoreableEnvelopeQuarantinedBeforeLaterProgress; real TestSupportedJSONBRepresentationsReallyPersist and TestUnstoreableJSONQuarantineAndProgress |
| Startup cancellation after successful Compose up leaked an unreported stack | Deferred failure cleanup covers endpoint discovery and manifest writes; independent bounded cleanup context; errors include project/evidence path and destruction failures | TestFailedStartupCleansUpAndReportsEvidence, original source RED via temporary overlay, corrected source GREEN |
| Readiness only checked connectivity while writes could fail | Processing health survives reader recreation and clears only when the failed operation succeeds | TestProcessingFailureSurvivesReaderRestart and TestReadinessFailsWhenPingWorksButWritingFails |
| Signal cancellation aborted active consumer work and discarded the result | Separate poll admission from active-work cancellation; bounded current-batch drain; report deadline/incomplete-work errors | TestStopPollingDrainsCurrentWrite, TestStopPollingDeadlineDoesNotClaimCompletion, TestConsumerDrainFinishesWriteOrReportsDeadline; real TestConsumerGracefulStopExitsSuccessfully |

Each issue had a failing regression before the corresponding passing verification.
All 90 top-level unit cases passed under the race detector; vet and all command
builds passed. The full real integration/recovery
suite passed in 119.678 seconds after the changes. Both new extreme numeric
representations and 64-level payloads persisted in PostgreSQL; invalid broker
payloads were quarantined before later valid records progressed. No second
review was dispatched: the native execution workflow uses the fresh review once,
then regression tests to verify the single fix pass.

## Deferred minor

Supervisor retry errors still lack useful bounded structured diagnostics.
Readiness and metrics expose failure, but the application does not yet log a
safe error classification and source coordinates for each failed operation.
This limits diagnosis; it does not change offset/storage ordering. Future logging
must avoid keys, payloads and credentialled DSNs.

## Boundaries reviewed explicitly

Production TLS/authentication hardening and database row-level isolation remain
outside this local reference. Disk/host loss, retention-expiry recovery, maximum
capacity and long-duration stability were not established. Reused IDs retain
first stored content. COPY/staging remains a disclosed experiment despite losing
to batch INSERT at batch100. Removing the compiler/generated source interfaces
is an approved breaking redesign. These boundaries are stated publicly rather
than treated as proven capabilities.

## Public evidence

The local measurements remain tied to clean source deefc81; later validation and
lifecycle fixes have not been re-benchmarked. Timing captures were not edited to
imply otherwise. README setup/curl/SQL commands, normal and outage demos, both
language callers and their byte-identical retry modes were exercised. All
maintained local Markdown links resolved. README/article rendering and both
Mermaid diagrams were inspected; a sequence-diagram semicolon syntax error was
caught and corrected. The benchmark figure is generated directly from saved
samples.

The final outage demo again reconciled three acknowledged IDs after database
restoration and cleaned only its own stack. Normal repository checkout retained;
no merge, push, package publication or article/social posting was performed.
