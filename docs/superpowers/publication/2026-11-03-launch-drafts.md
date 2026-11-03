# Publication drafts — prepare only

These drafts are unpublished. Replace the article URL after choosing a home for it. Review the first-person voice and add only personal context that actually happened.

## Primary article

Title: Building an event pipeline in Go—and testing what survives a crash

Alternate, focused on the measured surprise: COPY wasn't the fastest part of my Go event pipeline

Source: [article](../../article.md). Publish the full article on the author's blog first if a suitable home exists. An optional DEV cross-post can point its canonical URL there; see [DEV's editor/cross-posting instructions](https://dev.to/help/writing-editing-scheduling). Convert repository-relative code/results links to absolute links at the measured revision before posting. Confirm the branch and referenced commits have been pushed so readers can resolve them. Keep the complete sample ranges and environment caveats.

## Regular Hacker News submission

Title: Building a Go event pipeline and testing the database-commit crash window

URL: the final public article URL.

Optional author comment: I built a small HTTP → Redpanda → PostgreSQL collector to make acknowledgement and replay behavior inspectable. The repo includes real dependency-outage tests and a deterministic kill between database and offset commits. One result surprised me: with batches of 100, ordinary batch INSERT beat COPY/staging/merge. The local load runs reconcile every ID, but they are not a maximum-capacity result. I'm interested in feedback on the failure boundaries and benchmark method.

Do not call an article submission Show HN. [HN's guidelines](https://news.ycombinator.com/showhn.html) distinguish reading material from something people can try.

## Potential separate Show HN

Title: Show HN: IngestKit – break an event pipeline and reconcile what survived

URL: the public repository once this branch is available.

Description: A small Go collector with an isolated checkout demo and a database-outage demo. Run make demo-recovery to acknowledge events while PostgreSQL is stopped, restore it, and compare the same IDs. The integration suite also kills the consumer after database commit but before offset commit. Plain Python and Node callers demonstrate reuse. Requires local Docker and Go; no hosted account or generated SDK.

Use only when the runnable project is publicly accessible. Do not coordinate votes or promise popularity. The article and runnable project serve different submissions; avoid duplicative promotion.

## Resume / project description

Built a Go event collector using Redpanda and PostgreSQL, with broker-acknowledged HTTP ingestion, tenant-scoped idempotent storage, bounded retries and acknowledged quarantine. Verified replay using a deterministic crash between database and offset commits and automated dependency-outage/rebalance tests. Recorded reproducible local database and pipeline experiments with exact ID reconciliation, raw samples and environment manifests.

Resume bullet: Built and failure-tested a Go/Redpanda/PostgreSQL event pipeline; verified database-commit replay and tenant-scoped deduplication, and recorded reproducible local ingestion experiments with raw correctness evidence.

Do not add claims about production adoption, business impact, cloud cost, maximum throughput or popularity without new evidence.
