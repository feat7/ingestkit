# Publication drafts — updated 4 November 2026

These are local drafts. No article, repository release or social submission has
been published by this work. Check that the reviewed branch and referenced source
commits are available publicly before using these links. Add personal context only
if it happened; do not invent adoption, company impact or popularity.

## Article

Title: Building an event collector around the failure boundaries

Alternative: What HTTP 202 means in a self-hosted event pipeline

Source: [publication-ready Markdown draft](../../article.md). Choose the author's
canonical blog home, replace relative code/results/image links with public URLs,
and preview the responsive figures on that platform. Keep the measured revision,
three-repetition ranges, unknown deliveries and shared-host caveats beside claims.
If cross-posting, retain one canonical article URL rather than splitting the story
across competing copies. No hosting destination is selected or created here.

The concrete hook: a deployable collector plus an application outbox, two separate
crash windows, and evidence that changed the default from COPY/staging/merge to
batch INSERT. The value is being able to run, break and inspect it.

## Regular Hacker News article submission

Title: Building an event collector and testing its failure boundaries

URL: the final public article URL.

Optional author comment:

> I built a small Go/Redpanda/PostgreSQL event collector with a self-contained
> HTTPS deployment and a runnable transactional-outbox example. The repo tests
> crashes after SQL commit and after HTTP acknowledgement, then reconciles the
> same event IDs. A corrected 25-case study favored batch INSERT over my COPY/merge
> path by median, so I changed the default and reran it. All repetitions and limits
> remain in the captures, including ambiguous deliveries; these are local results,
> not maximum-capacity claims. I'd welcome feedback on the failure model and method.

A blog post belongs in a regular submission. The [Show HN guidelines](https://news.ycombinator.com/showhn.html)
reserve Show HN for something people can try, rather than reading material.

## Potential separate Show HN

Title: Show HN: IngestKit – a self-hosted event collector with crash/replay tests

URL: the public repository after this branch is made available.

Description draft:

> Deploy a small HTTPS event collector with Docker Compose, send stable UUIDs from
> Python or Node, and inspect PostgreSQL storage. The application outbox persists
> orders and pending events atomically and replays the same IDs after a dispatcher
> crash. Local demos and real-service tests stop dependencies and check accepted
> IDs, stored content and source progress. Deployment builds Go inside Docker;
> owned local demos/studies require Go. It is one host with finite retention, not HA.

Use only when readers can actually access and run the reviewed code. Prefer one
focused submission at a time; this file does not authorize posting, messaging,
vote coordination or account/hosting setup.

## Resume / project description

Built a self-hosted Go event collector with HTTPS, Redpanda buffering and
PostgreSQL storage; supplied copyable Python/Node clients and a transactional
outbox. Verified tenant-scoped deduplication and crash replay with real dependency
outages, restricted-role/restore tests and a reproducible 25-configuration study.
Changed the default SQL writer based on measured batch/payload/duplicate results.

Short bullet:

Built and failure-tested a self-hosted Go/Redpanda/PostgreSQL collector with HTTPS,
durable application outbox and replay-safe storage; recorded 25 reproducible
experiment configurations with raw samples, ID reconciliation and explicit limits.

No claim of production adoption, business impact, cloud cost, maximum throughput
or popularity is supported by the local repository evidence.
