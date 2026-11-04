"""Phone reading guides for the two source-backed event-story diagrams.

These explain the same responsibilities as event-journey.architecture.json and
worker-replay.sequence.json. They are teaching views, not measured packet traces.
Runtime source: 799106aeed58f6618bdc4b3711a84e11000fb0b5.
"""

from html import escape
from pathlib import Path

STYLE = """<style>
:root{--paper:#faf9f6;--ink:#18323c;--muted:#526671;--line:#b4c3c9;--blue:#205f72;--panel:#edf3f3}
@media(prefers-color-scheme:dark){:root{--paper:#12212b;--ink:#edf2f4;--muted:#bbccd3;--line:#526671;--blue:#7bc6d6;--panel:#1a303c}}
text{font-family:Arial,sans-serif;fill:var(--ink)}
.title{font-size:21px;font-weight:700}.heading{font-size:18px;font-weight:700}
.copy{font-size:16px}.small{font-size:13px;fill:var(--muted)}
.card{fill:var(--panel);stroke:var(--line);stroke-width:1.5}
.arrow{stroke:var(--blue);stroke-width:2;fill:none}
</style>"""


def text(x, y, label, kind="copy"):
    return f'<text x="{x}" y="{y}" class="{kind}">{escape(label)}</text>'


def page(title, description, height, parts):
    return "\n".join(
        [
            f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 360 {height}" role="img" aria-labelledby="title desc">',
            f'<title id="title">{escape(title)}</title><desc id="desc">{escape(description)}</desc>',
            STYLE,
            f'<rect width="360" height="{height}" rx="12" fill="var(--paper)"/>',
            *parts,
            "</svg>",
            "",
        ]
    )


def arrow(start, end, label):
    return [
        f'<path d="M180 {start}V{end}" class="arrow"/>',
        f'<path d="M175 {end-6}L180 {end}L185 {end-6}" class="arrow"/>',
        text(194, (start + end) // 2 + 4, label, "small"),
    ]


def journey():
    parts = [
        text(20, 34, "Follow one event", "title"),
        text(20, 61, "Keep the same event ID on retry.", "copy"),
    ]
    rows = [
        ("1. Your application", "Saves the ID and event body."),
        ("2. HTTPS endpoint", "Caddy and Go check the request."),
        ("3. Redpanda queue", "Holds the event for the worker."),
        ("4. Database worker", "Retries when a write fails."),
        ("5. PostgreSQL", "One row per tenant + event ID."),
    ]
    labels = ["send", "append", "read later", "save"]
    for i, (heading, body) in enumerate(rows):
        y = 90 + i * 112
        parts += [
            f'<rect x="20" y="{y}" width="320" height="76" rx="12" class="card"/>',
            text(36, y + 29, heading, "heading"),
            text(36, y + 57, body),
        ]
        if i < 4:
            parts += arrow(y + 80, y + 108, labels[i])
    parts += [
        text(20, 659, "202 confirms queue acceptance.", "copy"),
        text(20, 685, "The database write runs separately.", "copy"),
        text(20, 724, "Queue recovery needs retained data and disks.", "small"),
        text(20, 746, "Source 799106a; full diagram linked.", "small"),
    ]
    return page(
        "From one event to a saved row",
        "The application sends a saved event ID over HTTPS. Redpanda holds it while a worker saves a PostgreSQL row.",
        770,
        parts,
    )


def replay():
    parts = [
        text(20, 34, "One row after a restart", "title"),
        text(20, 63, "The worker saves the row first.", "copy"),
    ]
    for y, heading, lines in [
        (
            96,
            "Before the crash",
            [
                "PostgreSQL has the saved row.",
                "The queue position is not saved.",
                "The worker stops here.",
            ],
        ),
        (
            350,
            "After restart",
            [
                "The worker reads the same ID.",
                "PostgreSQL keeps the existing row.",
                "The worker saves its queue position.",
            ],
        ),
    ]:
        parts += [
            f'<rect x="20" y="{y}" width="320" height="182" rx="12" class="card"/>',
            text(36, y + 34, heading, "heading"),
        ]
        parts += [text(36, y + 74 + i * 30, line) for i, line in enumerate(lines)]
    parts += arrow(286, 338, "restart")
    parts += [
        text(20, 576, "Unique key: tenant + event ID.", "copy"),
        text(20, 605, "A repeat does not update the saved content.", "small"),
        text(20, 640, "Source 799106a; full sequence linked.", "small"),
    ]
    return page(
        "A replay keeps the existing row",
        "After a worker stops between SQL commit and queue-position commit, the same event is read again. Its unique key keeps one row.",
        664,
        parts,
    )


if __name__ == "__main__":
    output = Path(__file__).resolve().parents[1] / "assets"
    (output / "event-journey-mobile.svg").write_text(journey())
    (output / "worker-replay-mobile.svg").write_text(replay())
