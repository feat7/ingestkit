"""Generate reproducible study figures; Python + matplotlib are developer tools."""

import argparse
import json
from pathlib import Path
import statistics
import textwrap
import matplotlib

matplotlib.use("Agg")
matplotlib.rcParams.update(
    {
        "svg.hashsalt": "ingestkit-study-v1",
        "svg.fonttype": "path",
        "font.family": "DejaVu Sans",
        "font.size": 13,
        "axes.titlesize": 14,
        "axes.labelsize": 12,
        "xtick.labelsize": 11,
        "ytick.labelsize": 11,
        "text.color": "#18323c",
        "axes.labelcolor": "#18323c",
        "axes.edgecolor": "#73818b",
        "xtick.color": "#18323c",
        "ytick.color": "#18323c",
    }
)
import matplotlib.pyplot as plt
from matplotlib.lines import Line2D
from audit_study import audit, read_artifact

FS = 1.0
PAPER = "#faf9f6"
COLORS = ["#73818b", "#205f72", "#ac4d36"]
METHODS = ["insert", "batch-insert", "copy-merge"]
LABELS = ["One INSERT", "Batch INSERT", "COPY + merge"]


def canvas(rows, height):
    fig, axes = plt.subplots(rows, 1, figsize=(6.5, height * FS), layout="constrained")
    fig.patch.set_facecolor(PAPER)
    if rows == 1:
        axes = [axes]
    for ax in axes:
        ax.set_facecolor(PAPER)
        ax.spines[["top", "right"]].set_visible(False)
        ax.grid(alpha=0.18, zorder=0)
    return fig, axes


def caption(fig, text, fontsize):
    if FS > 1:
        text = "\n".join(textwrap.fill(line, width=57) for line in text.splitlines())
    fig.supxlabel(text, fontsize=fontsize)


def save(fig, name, args, description):
    target = args.output / (name + args.variant + ".svg")
    fig.savefig(
        target,
        metadata={
            "Date": None,
            "Creator": "IngestKit study figure generator",
            "Description": description,
        },
    )
    target.write_text(
        "\n".join(line.rstrip() for line in target.read_text().splitlines()) + "\n"
    )
    if args.preview_dir:
        fig.savefig(args.preview_dir / (name + args.variant + ".png"), dpi=140)
    plt.close(fig)


def database(index, rows, args, stamp):
    fig, axes = canvas(3, 12.5)
    panels = [
        (
            "BatchSize",
            [1, 10, 50, 100, 500, 1000],
            [rows["db-batch-" + str(b)] for b in (1, 10, 50, 100, 500, 1000)],
            "Batch size · 256 B, no duplicates",
            "Events per transaction",
        ),
        (
            "PayloadBytes",
            [256, 1024, 16384],
            [rows["db-batch-100"], rows["db-payload-1024"], rows["db-payload-16384"]],
            "Payload size · batch 100, no duplicates",
            "Encoded JSON payload bytes",
        ),
        (
            "DuplicateFraction",
            [0, 0.1, 0.5, 0.9],
            [
                rows["db-batch-100"],
                rows["db-duplicates-10"],
                rows["db-duplicates-50"],
                rows["db-duplicates-90"],
            ],
            "Repeated IDs · batch 100, 256 B",
            "Duplicate submissions (%)",
        ),
    ]
    for ax, (field, xs, selected, title, xlabel) in zip(axes, panels):
        for mi, method in enumerate(METHODS):
            for x, row in zip(xs, selected):
                values = [
                    r["submitted"] * 1e9 / r["elapsed_ns"]
                    for r in row["Summary"]["runs"]
                    if r["method"] == method
                ]
                median = statistics.median(values)
                ax.errorbar(
                    [x],
                    [median],
                    yerr=[[median - min(values)], [max(values) - median]],
                    color=COLORS[mi],
                    capsize=3,
                    fmt="_",
                    markersize=11,
                    zorder=3,
                )
                offsets = [
                    (
                        x * (1 + v * 0.035)
                        if field != "DuplicateFraction"
                        else x + v * 0.012
                    )
                    for v in (-1, 0, 1)
                ]
                ax.scatter(
                    offsets,
                    values,
                    s=22,
                    color=COLORS[mi],
                    marker=["o", "s", "^"][mi],
                    alpha=0.8,
                    zorder=4,
                )
        if field != "DuplicateFraction":
            ax.set_xscale("log")
        ax.set_yscale("log")
        ax.set_xticks(
            xs,
            [
                str(round(x * 100)) if field == "DuplicateFraction" else str(x)
                for x in xs
            ],
        )
        ax.minorticks_off()
        ax.set(title=title, xlabel=xlabel, ylabel="Submitted events/s (log)")
    axes[0].legend(
        [
            Line2D([], [], marker=m, color=c, linestyle="")
            for m, c in zip(["o", "s", "^"], COLORS)
        ],
        LABELS,
        ncol=(1 if FS > 1 else 3),
        fontsize=int(10 * FS),
        loc="upper left",
    )
    fig.suptitle(
        "Batch INSERT won these workloads", fontsize=int(17 * FS), weight="bold"
    )
    caption(
        fig,
        "5,000 submissions/method · 3 measured repetitions\nDots = repetitions; bars = min–max, center = median.\nDuplicate cases insert 5,000 / 4,500 / 2,500 / 500 unique IDs.\n"
        + stamp
        + "\nSame indexed table, durable commits, shared local host.",
        fontsize=int(10 * FS),
    )
    save(
        fig,
        "database-batching",
        args,
        "All 11 database configurations and 99 method/repetition rows. Submitted rate is not unique insertion rate in duplicate cases. "
        + stamp,
    )


def pipeline(index, rows, args, stamp):
    selected = [r for r in index["Cases"] if r["Case"]["Kind"] == "pipeline"]
    names = [
        "batch 1 · 1k/s",
        "batch 10 · 1k/s",
        "batch 100 · 1k/s",
        "100/s · batch 100",
        "5k/s · batch 100",
        "10k/s · batch 100",
        "1 KiB · batch 10",
        "16 KiB · batch 10",
        "burst · batch 100",
        "8 clients · batch 1",
        "64 clients · batch 1",
    ]
    fig, axes = canvas(2, 14)
    for i, row in enumerate(selected):
        c = row["Case"]
        axes[0].scatter([c["Rate"]], [i], marker="|", s=170, color="#a1aab0", zorder=2)
        for rep, run in enumerate(row["Summary"]["runs"]):
            y = i + (rep - 1) * 0.2
            limited = run["outcome"] == "limited"
            for x, color, marker in [
                (run["AttemptedInWindow"] * 1e9 / run["WindowNS"], "#73818b", "s"),
                (run["AcceptedEPS"], "#205f72", "o"),
            ]:
                axes[0].scatter(
                    [x],
                    [y],
                    s=32,
                    marker=marker,
                    edgecolors=color,
                    facecolors=PAPER if limited else color,
                    zorder=3,
                )
            for field, color, marker in [
                ("ack_latency", "#ac4d36", "s"),
                ("receipt_to_observed_storage_upper_bound", "#205f72", "o"),
            ]:
                axes[1].scatter(
                    [run[field]["p95_ns"] / 1e6],
                    [y],
                    s=32,
                    marker=marker,
                    edgecolors=color,
                    facecolors=PAPER if limited else color,
                    zorder=3,
                )
    for ax in axes:
        ax.set_yticks(range(len(names)), names)
        ax.invert_yaxis()
        ax.set_xscale("log")
        ax.grid(axis="x", alpha=0.2)
        ax.grid(axis="y", visible=False)
    axes[0].set(
        xlabel=(
            "Events/s in 10-second window\n(log scale)"
            if FS > 1
            else "Events/s inside the 10-second window (log)"
        )
    )
    axes[1].set(xlabel="p95 latency · milliseconds (log)")
    axes[0].legend(
        [
            Line2D([], [], marker=m, color=c, linestyle="")
            for m, c in [("|", "#a1aab0"), ("s", "#73818b"), ("o", "#205f72")]
        ],
        ["Target", "Attempted", "ACK"],
        title="Offered / attempted / ACK",
        title_fontsize=int(12 * FS),
        ncol=(2 if FS > 1 else 3),
        fontsize=int(10 * FS),
        loc="lower left",
        bbox_to_anchor=(0, 1.01),
        borderaxespad=0,
    )
    axes[1].set_xlim(10, 500)
    axes[1].set_xticks(
        [10, 20, 50, 100, 200, 500], ["10", "20", "50", "100", "200", "500"]
    )
    axes[1].minorticks_off()
    axes[1].legend(
        [
            Line2D([], [], marker=m, color=c, linestyle="")
            for m, c in [("s", "#ac4d36"), ("o", "#205f72")]
        ],
        ["ACK/request", "SQL/event upper bound"],
        title="p95 latency populations",
        title_fontsize=int(12 * FS),
        ncol=(1 if FS > 1 else 2),
        fontsize=int(10 * FS),
        loc="lower left",
        bbox_to_anchor=(0, 1.01),
        borderaxespad=0,
    )
    fig.suptitle(
        "Offered load is not achieved capacity", fontsize=int(17 * FS), weight="bold"
    )
    caption(
        fig,
        "3 repetitions/case · hollow markers = reported limits\nAll 11 configurations; every outcome retained.\nACK: successful requests. SQL: first observed committed events;\n50 ms polling + query delay, shared deployment clock.\n"
        + stamp
        + "\nPrivate-CA HTTPS; generator/observer share the host.",
        fontsize=int(10 * FS),
    )
    save(
        fig,
        "pipeline-load",
        args,
        "All 11 pipeline configurations and 33 measured windows. Hollow markers preserve late schedule, observer, rejection, ambiguity and tail limits. SQL latency is an observation upper bound, not physical commit time. "
        + stamp,
    )


def recovery(index, rows, args, stamp):
    fig, axes = canvas(3, 12.7)
    names = ["recovery-database", "recovery-consumer", "recovery-outbox"]
    for ax, name, title in zip(
        axes,
        names,
        [
            "PostgreSQL outage",
            "Consumer kill and restart",
            "Broker outage + durable application outbox",
        ],
    ):
        row = rows[name]
        capture = read_artifact(args.study, row["Artifact"])
        for run in capture["Report"]["Runs"]:
            rep = run["Repetition"]
            color = COLORS[rep - 1]
            if run.get("Pipeline"):
                points = run["Pipeline"]["backlog"]
                xs = [p["elapsed_ns"] / 1e9 for p in points]
                ys = [
                    (
                        p["PendingAcknowledged"]
                        if p["ObservationAvailable"]
                        else float("nan")
                    )
                    for p in points
                ]
            else:
                xs = [p["ElapsedNS"] / 1e9 for p in run["Outbox"]]
                ys = [p["Pending"] for p in run["Outbox"]]
            if run.get("Pipeline"):
                ax.plot(
                    xs,
                    ys,
                    color=color,
                    label="rep " + str(rep),
                    linewidth=1.4,
                    alpha=0.9,
                    marker=".",
                    markersize=2,
                    linestyle=["-", "--", ":"][rep - 1],
                )
            else:
                ax.scatter(xs, ys, color=color, label="rep " + str(rep), s=24)
            faults = {f["Action"]: f["ElapsedNS"] / 1e9 for f in run["Faults"]}
            for action, style in [("stopped", ":"), ("started", "--")]:
                ax.axvline(
                    faults[action],
                    color=color,
                    linestyle=style,
                    linewidth=0.9,
                    alpha=0.6,
                )
        ax.set(
            title=title,
            xlabel="Seconds from measured workload start",
            ylabel=(
                "Pending acknowledged events"
                if name != "recovery-outbox"
                else "Pending application outbox rows"
            ),
            ylim=(-10, None),
        )
    axes[0].set_title("")
    axes[0].legend(
        title="PostgreSQL outage",
        title_fontsize=int(14 * FS),
        fontsize=int(10 * FS),
        ncol=3,
        loc="lower left",
        bbox_to_anchor=(0, 1.01),
        borderaxespad=0,
    )
    fig.suptitle(
        "Recovery is a separate experiment", fontsize=int(17 * FS), weight="bold"
    )
    caption(
        fig,
        "All 3 repetitions per failure · dots are observations\nDotted line: stop/kill command completed; dashed: start completed.\nSQL gaps are blank; lines join observations, not exact commits.\nOutbox retains 100 stable IDs across the >5 s failed attempt.\n"
        + stamp
        + "\nFinal independent ID/content reconciliation is mandatory.",
        fontsize=int(10 * FS),
    )
    save(
        fig,
        "recovery",
        args,
        "Three availability experiments with three repetitions, actual Docker control completion times and explicit SQL observation gaps. Pending acknowledgement observations are not exact physical-commit backlogs. "
        + stamp,
    )


def main():
    global FS
    root = Path(__file__).resolve().parent.parent
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--study", type=Path, default=root / "benchmarks/results/selfhost-2026-11-04"
    )
    parser.add_argument("--output", type=Path, default=root / "docs/assets")
    parser.add_argument("--preview-dir", type=Path)
    args = parser.parse_args()
    audit(args.study)
    index = json.loads((args.study / "index.json").read_text())
    rows = {r["Case"]["ID"]: r for r in index["Cases"]}
    args.output.mkdir(parents=True, exist_ok=True)
    if args.preview_dir:
        args.preview_dir.mkdir(parents=True, exist_ok=True)
    stamp = (
        "Source "
        + index["Identity"]["SourceSHA"][:7]
        + " · study "
        + args.study.name
        + " · seed "
        + str(index["Identity"]["Seed"])
    )
    base = matplotlib.rcParams.copy()
    for FS, args.variant in [(1.0, ""), (1.35, "-mobile")]:
        for key in (
            "font.size",
            "axes.titlesize",
            "axes.labelsize",
            "xtick.labelsize",
            "ytick.labelsize",
        ):
            matplotlib.rcParams[key] = float(base[key]) * FS
        database(index, rows, args, stamp)
        pipeline(index, rows, args, stamp)
        recovery(index, rows, args, stamp)
    print(
        "Generated desktop/mobile database, pipeline and recovery figures from audited study "
        + str(args.study)
    )


if __name__ == "__main__":
    main()
