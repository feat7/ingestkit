# Figure sources and reproduction

The diagrams describe source revision [213e4cc](../../benchmarks/results/selfhost-2026-11-04/manifest-1.json).
Their JSON sources cite the actual gateway, API, consumer, SQL and migration code.
The charts read the [audited study index](../../benchmarks/results/selfhost-2026-11-04/index.json),
including every repetition and reported limit. No chart uses the historical
fsync-bypassed pipeline captures.

## Generate the charts

The recorded plotting environment was Python3.12.14 and matplotlib3.11.2.
These are development tools; deployed services and copied HTTP clients do not
need matplotlib. From the repository root, in a Python3.12 environment:

~~~sh
python3 -m venv .venv
.venv/bin/pip install matplotlib==3.11.2
.venv/bin/python benchmarks/plot_results.py \
  --study benchmarks/results/selfhost-2026-11-04 --output docs/assets
python3 docs/figures/render_mobile_guides.py
~~~

Optional `--preview-dir tmp/figure-previews` also writes PNGs for inspection.
The generator independently audits the captures before plotting, fixes SVG IDs,
omits timestamps, embeds portable font outlines and writes desktop/mobile layouts
from the same measurements. Mobile layouts increase type size and stack legends;
no repetitions or limits disappear. The compact deployment/delivery guides are
source-cited typographic companions to the full Archify diagrams.

## Reproduce the full diagrams

[deployment.architecture.json](deployment.architecture.json) and
[delivery.sequence.json](delivery.sequence.json) were generated with Archify3.2.0,
then passed its showcase validation, strict artifact and real-browser gates.
The HTML and browser receipts live in the ignored `.archify/` directory.
With an installed Archify CLI, set `ARCHIFY_CLI` to its absolute
`bin/archify.mjs` path and run from this repository:

~~~sh
node "$ARCHIFY_CLI" finalize architecture docs/figures/deployment.architecture.json \
  .archify/architecture-deployment-20261104-041835/deployment.html \
  --repo-root "$PWD" --quality showcase --json
node "$ARCHIFY_CLI" finalize sequence docs/figures/delivery.sequence.json \
  .archify/sequence-delivery-20261104-041835/delivery.html \
  --repo-root "$PWD" --quality showcase --json
~~~

Open each resulting HTML and choose **Export → SVG · Auto**. Save the full clean
exports as `docs/assets/deployment.svg` and `docs/assets/delivery.svg`. They include
automatic light/dark styling and exclude viewer controls. The checked repository
identity is the original SSH remote; source links are intentionally local-only.
A fresh changed candidate needs a new browser-evidence directory as documented by
Archify. The compact reading guides are generated separately by
[render_mobile_guides.py](render_mobile_guides.py), not described as packet traces.

[Verification record and asset hashes](verification.json) cover the checked exports,
regeneration and visual review at800px and390px. Charts use an opaque light panel
for contrast on either page background; the diagram exports adapt to host theme.

## Inspect the results

<picture><source media="(max-width:600px)" srcset="../assets/deployment-mobile.svg"><img src="../assets/deployment.svg" alt="Only Caddy publishes ports; the collector dependencies stay private on one host."></picture>

[Full deployment diagram](../assets/deployment.svg). Persistent volumes and the
single-host failure domain matter as much as the arrows; see the
[deployment guide](../../deploy/README.md).

<picture><source media="(max-width:600px)" srcset="../assets/delivery-mobile.svg"><img src="../assets/delivery.svg" alt="202 acknowledges the broker; SQL commit precedes source-offset commit; a crash in between replays the same UUID."></picture>

[Full sequence](../assets/delivery.svg). The second segment is an alternative
crash path, not a claim that an already committed offset replays. HTTP response
and consumer processing are concurrent after broker acceptance.

<picture><source media="(max-width:600px)" srcset="../assets/database-batching-mobile.svg"><img src="../assets/database-batching.svg" alt="All 99 database method/repetition measurements, with batch-size, payload-size and duplicate-ID controls."></picture>

Submitted/s differs from inserted unique/s for duplicate workloads. Dots retain
all three repetitions; min–max/median marks are descriptive, not confidence intervals.

<picture><source media="(max-width:600px)" srcset="../assets/pipeline-load-mobile.svg"><img src="../assets/pipeline-load.svg" alt="All 33 HTTPS windows: target versus generated/acknowledged load and p95 ACK versus observed storage latency."></picture>

Hollow markers retain limits, including ordinary finite-window tails. ACK samples
are successful requests; SQL samples are first-observed committed events. The
latter include polling/query delay. Neither panel establishes maximum capacity.

<picture><source media="(max-width:600px)" srcset="../assets/recovery-mobile.svg"><img src="../assets/recovery.svg" alt="All three repetitions of database outage, consumer restart and outbox replay with actual control timestamps."></picture>

SQL-observer gaps remain blank. Fault lines mark Docker control completion, not
an exact service-availability transition. Outbox points are state snapshots;
intermediate changes are not fabricated. Pending acknowledged counts are based
on observation, not an instantaneous database backlog. Read the
[methodology and all outcomes](../../benchmarks/results/selfhost-2026-11-04/README.md)
with the figures.
