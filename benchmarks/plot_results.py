"""Optional figure generator: Python + matplotlib; no service dependency."""
import json
from pathlib import Path
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
root = Path(__file__).resolve().parent
results = root / 'results/local-2026-11-03'
fig, axes = plt.subplots(1, 2, figsize=(11.5, 4.4), layout='constrained')
fig.patch.set_facecolor('#faf9f6')
for ax in axes:
    ax.set_facecolor('#faf9f6')
    ax.spines[['top', 'right']].set_visible(False)
    ax.grid(axis='y', alpha=.18)
runs = json.loads((results / 'db-unique/database.json').read_text())['runs']
for i, method in enumerate(['insert', 'batch-insert', 'copy-merge']):
    values = [v['elapsed_ns'] / 1e9 for v in runs if v['method'] == method]
    axes[0].scatter([i - .06, i, i + .06], values, s=42, color='#205f72', zorder=3)
axes[0].set(xticks=range(3), xticklabels=['Single INSERT', 'Batch INSERT', 'COPY + merge'], yscale='log', ylabel='Seconds to submit 10,000 events (log scale)', title='Small batches: COPY/staging did not win')
for i, pattern in enumerate(['steady', 'burst']):
    runs = json.loads((results / pattern / 'pipeline.json').read_text())['runs']
    for j, v in enumerate(runs):
        axes[1].scatter(i + (j - 1) * .09, v['receipt_to_observed_storage_upper_bound']['p95_ns'] / 1e6, s=50, color='#ac4d36', zorder=3)
        axes[1].annotate(str(j + 1), (i + (j - 1) * .09, v['receipt_to_observed_storage_upper_bound']['p95_ns'] / 1e6), xytext=(5, 4), textcoords='offset points', fontsize=9)
axes[1].set(xticks=[0, 1], xticklabels=['Steady: 1,000/s', 'Burst: 4,000/s active'], xlim=(-.5, 1.5), ylim=(0, 310), ylabel='Receipt to visible committed row, p95 (ms)', title='All three repetitions matter')
fig.suptitle('Local evidence · 256-byte payload · batch 100 · shared M1 Max / OrbStack', fontsize=12)
fig.supxlabel('Each dot is one measured repetition. Storage visibility includes 50 ms polling and query time.', fontsize=9)
fig.savefig(root.parent / 'docs/assets/experiments.svg', metadata={'Date': None, 'Creator': 'IngestKit benchmark figure'})
fig.savefig('/private/tmp/ingestkit-experiments.png', dpi=140)
