# Figures that explain an event and the measurements

The event journey answers where an accepted event waits. The worker replay shows
why a crash after SQL commit still leaves one row. The detailed deployment and
request-sequence views are available when you need the service or packet order.
Each has a separate phone reading guide.

- [Event journey](../assets/event-journey.svg) · [phone](../assets/event-journey-mobile.svg)
- [Worker replay](../assets/worker-replay.svg) · [phone](../assets/worker-replay-mobile.svg)
- [Deployment](../assets/deployment.svg) · [phone](../assets/deployment-mobile.svg)
- [Request sequence](../assets/delivery.svg) · [phone](../assets/delivery-mobile.svg)

The [connection-pressure figure](../assets/connection-pressure.svg) shows every
repetition in the fixed cap sweep, with configured caps separated from observed
TCP peaks. The [acceptance/storage figure](../assets/acceptance-storage.svg) follows
all three 50k-offered repetitions through acknowledgements, observed rows and the
final ID/content check. Its lower panels show sampled queue-position lag. Gaps
mean unavailable probes. It does not draw an exact database queue from an
observed-pending count. Both figures have phone layouts, linked from the
[measurement guide](../../benchmarks/README.md).

## Regenerate measured figures

The plotter audits the immutable schema 2 bundle before reading its series.
Use Python 3.12+ with Matplotlib 3.11.2 for the recorded export:

~~~sh
python3 benchmarks/plot_connections.py
~~~

The default input is the retained connection bundle. `--study PATH` selects an
explicit connection-study bundle; `--out PATH` selects an output directory. `--preview PATH`
also writes PNGs for visual review. The plotter needs Git history containing the
measured source revision. Never replace failed timing with a zero-valued point.

## Regenerate source-backed diagrams

Set `ARCHIFY_CLI` to the installed Archify 3.2.0 `bin/archify.mjs` path:

~~~sh
node "$ARCHIFY_CLI" finalize architecture docs/figures/event-journey.architecture.json \
  .archify/event-journey/event-journey.html --repo-root "$PWD" --quality showcase --json
node "$ARCHIFY_CLI" finalize sequence docs/figures/worker-replay.sequence.json \
  .archify/worker-replay/worker-replay.html --repo-root "$PWD" --quality showcase --json
node "$ARCHIFY_CLI" finalize architecture docs/figures/deployment.architecture.json \
  .archify/deployment/deployment.html --repo-root "$PWD" --quality showcase --json
node "$ARCHIFY_CLI" finalize sequence docs/figures/delivery.sequence.json \
  .archify/delivery/delivery.html --repo-root "$PWD" --quality showcase --json
python3 docs/figures/render_story_guides.py
python3 docs/figures/render_mobile_guides.py
~~~

Open each HTML and export SVG Auto to its matching file in `docs/assets/`.
Check a changed diagram in the browser before replacing an export. The phone
files are reading guides produced by the two Python generators; they explain
the same behavior without shrinking a full packet trace onto a phone.

[manifest.json](manifest.json) lists current asset/source/data hashes and
validated source ranges. The eight retained diagram exports originated at
`799106a` or `213e4cc`. Their ranges were checked against `043f584`, including
publisher instrumentation and client deadline changes. That source validation is
separate from their original export provenance. The measured figures use the
new bundle; old study receipts and charts were archived intact.
