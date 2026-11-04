# Publication drafts (updated 4 November 2026)

Choose a blog home before posting the article. These drafts do not publish it,
create a release or submit anything to social sites. Check public code/results
links before posting. Add personal context only when it happened; the repository
does not establish adoption, company impact or popularity.

## Article

Title: What happens after an event collector says "accepted"?

Alternative: Keeping an event through a database outage and a worker restart

The [article draft](../../article.md) follows an order event from the application
to PostgreSQL, then explains the retry gaps and the write-method experiment.
Replace relative code/results/image links with public URLs for the chosen blog,
and check its image rendering on a phone. Keep the measured revision, all three
runs, uncertain outcomes and shared-host limits beside the claims. If cross-posting,
choose one canonical article URL.

The story has a runnable deployment, an application outbox and two tested crash
gaps. The database experiment also changed the implementation: batched INSERT
replaced COPY/staging/merge after the measured comparison.

## Hacker News article submission

Title: What happens after an event collector says "accepted"?

URL: the final public article URL.

Optional author comment:

> I built a small Go event collector with an HTTPS API, Redpanda queue and
> PostgreSQL store. The article follows an order event through a database outage
> and two crash gaps. The repo includes a Docker Compose deployment and a small
> order/outbox app you can run. Tests stop the worker after SQL commit and stop
> the sender after HTTP acknowledgement, then check the stored IDs. A 25-case
> study found a lower median for batch INSERT than for my COPY/merge path in
> all 11 database configurations, so I changed the writer and ran the study
> again. The results include all runs and unknown deliveries. I'd welcome
> feedback on the retry behavior and experiment design.

Use a regular submission for the article. [Show HN](https://news.ycombinator.com/showhn.html)
is for a project readers can try.

## Separate Show HN submission

Title: Show HN: IngestKit, a self-hosted event collector with crash/replay tests

URL: the public repository containing the reviewed code.

Description draft:

> Run an HTTPS event collector with Docker Compose, send events from Python or
> Node, and find the rows in PostgreSQL. The included order/outbox app saves
> orders and pending events together, then resends the same IDs after a crash.
> Demos and real-service tests stop dependencies and check which events reached
> storage. Go builds inside Docker on the deployment host; local demos and
> studies need Go installed. The server uses one host with finite queue retention.

Use this only when the reviewed code is publicly accessible and runnable.
Submit one focused story at a time. Posting, messages, vote coordination and
hosting/account setup need their own explicit request.

## Resume or project description

Built a self-hosted Go event collector with HTTPS, a Redpanda queue and PostgreSQL
storage, plus reusable Python/Node clients and a durable application outbox.
Tested database outages, writer permissions, restore and crashes during retry;
verified that replay kept one row per tenant/event ID. Ran 25 controlled experiment
configurations and changed the SQL writer based on the batch, payload and
repeated-ID results.

Short bullet:

Built and tested a Go/Redpanda/PostgreSQL event collector with a Docker Compose
HTTPS deployment and durable outbox; verified crash replay and recorded 25
experiment configurations with raw samples and explicit limits.

These local results do not support a claim of production adoption, business
impact, cloud cost, maximum throughput or popularity.
