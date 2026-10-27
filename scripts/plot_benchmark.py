#!/usr/bin/env python3
"""Publication/exportable PNG and SVG comparisons; medians across repetitions."""
import json
import os
from pathlib import Path
import statistics
import sys
os.environ.setdefault('MPLCONFIGDIR', str(Path(__file__).resolve().parents[1] / '.cache/matplotlib'))
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

directory = Path(sys.argv[1])
results = [row for mode in ('sql', 'redis_sql') for row in json.loads((directory / (mode + '.json')).read_text())]
fig, axes = plt.subplots(2, 3, figsize=(15, 8), constrained_layout=True)
for row_index, workload in enumerate(('hot', 'disjoint')):
    for mode in ('sql', 'redis_sql'):
        cases = [r for r in results if r['mode'] == mode and r['workload'] == workload]
        levels = sorted({r['concurrency'] for r in cases})
        for column, (label, metric) in enumerate([
            ('Accepted rides / second', lambda r: r['rideThroughput']),
            ('Accepted request p95 (ms)', lambda r: r['latency']['accepted']['p95']),
            ('SQL acceptance attempts / request', lambda r: r['metrics']['sqlAttempts'] / r['attempts']),
        ]):
            groups = [[metric(r) for r in cases if r['concurrency'] == level] for level in levels]
            values = [statistics.median(v) for v in groups]
            axes[row_index, column].plot(levels, values, marker='o', label=mode)
            axes[row_index, column].fill_between(levels, [min(v) for v in groups], [max(v) for v in groups], alpha=.12)
            axes[row_index, column].set(title=workload + ': ' + label, xlabel='Concurrent drivers', ylabel=label)
            axes[row_index, column].grid(alpha=.25)
            axes[row_index, column].legend()
fig.suptitle('Acceptance baseline — median and min/max across repetitions; shared host')
for extension in ('png', 'svg'):
    fig.savefig(directory / ('comparison.' + extension), dpi=160)
plt.close(fig)
