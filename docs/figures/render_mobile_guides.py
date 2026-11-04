"""Compact reading guides for the linked, source-backed Archify diagrams.

These are typographic companions, not additional packet traces. The full source
ranges are recorded in deployment.architecture.json and delivery.sequence.json.
Delivery source evidence:
internal/api/ingest.go:45-53; internal/messaging/process.go:14-30;
internal/persistence/events.go:22-39; migrations/showcase/000001_events.up.sql:2-12
at 213e4cc50523882f0d0cf7533d064494387d3207.
"""

from pathlib import Path

SVG = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 360 840" role="img" aria-labelledby="title desc">
<title id="title">Three delivery boundaries</title>
<desc id="desc">202 acknowledges the broker. SQL commits a replay-safe row. The consumer commits its offset after SQL succeeds. HTTP response and consumer progress run concurrently.</desc>
<style>
:root{--paper:#faf9f6;--ink:#18323c;--muted:#526671;--line:#b4c3c9;--blue:#205f72;--panel:#edf3f3}
@media(prefers-color-scheme:dark){:root{--paper:#12212b;--ink:#edf2f4;--muted:#bbccd3;--line:#526671;--blue:#7bc6d6;--panel:#1a303c}}
text{font-family:Arial,sans-serif;fill:var(--ink)}.small{font-size:14px;fill:var(--muted)}.heading{font-size:21px;font-weight:700}.copy{font-size:16px}.card{fill:var(--panel);stroke:var(--line);stroke-width:1.5}.step{font-size:14px;font-weight:700;fill:var(--blue)}.arrow{stroke:var(--blue);stroke-width:2;fill:none}
</style>
<defs><marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0 0L8 4L0 8" fill="var(--blue)"/></marker></defs>
<rect width="360" height="840" rx="12" fill="var(--paper)"/>
<text x="20" y="34" class="heading">Three delivery boundaries</text>
<text x="20" y="61" class="small">Same event UUID across retries.</text>
<rect x="20" y="92" width="320" height="156" rx="12" class="card"/>
<text x="38" y="122" class="step">1 · ACCEPTANCE</text>
<text x="38" y="154" class="heading">Broker ACK → HTTP 202</text>
<text x="38" y="185" class="copy">The broker acknowledged the event.</text>
<text x="38" y="212" class="small">PostgreSQL may still be pending.</text>
<path d="M180 254V297" class="arrow" marker-end="url(#arrow)"/>
<text x="194" y="278" class="small">async store</text>
<rect x="20" y="308" width="320" height="156" rx="12" class="card"/>
<text x="38" y="338" class="step">2 · STORAGE</text>
<text x="38" y="370" class="heading">SQL commit</text>
<text x="38" y="401" class="copy">One atomic batch, one row per ID.</text>
<text x="38" y="428" class="small">Crash now? Replay the same UUID.</text>
<path d="M180 470V513" class="arrow" marker-end="url(#arrow)"/>
<text x="194" y="494" class="small">after SQL</text>
<rect x="20" y="524" width="320" height="156" rx="12" class="card"/>
<text x="38" y="554" class="step">3 · PROGRESS</text>
<text x="38" y="586" class="heading">Commit source offset</text>
<text x="38" y="617" class="copy">The consumer advances after SQL.</text>
<text x="38" y="644" class="small">Replay keeps one row per tenant + UUID.</text>
<text x="20" y="720" class="small">HTTP response and consumer progress</text>
<text x="20" y="742" class="small">run concurrently after broker acceptance.</text>
<text x="20" y="777" class="small">Kafka + PostgreSQL are separate transactions.</text>
<text x="20" y="811" class="small">Source 213e4cc · full sequence linked beside guide</text>
</svg>"""


def deployment_guide():
    # Typography-first companion to deployment.architecture.json, same pinned sources.
    style = SVG[SVG.index("<style>") : SVG.index("</style>") + 8]
    parts = [
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 360 850" role="img" aria-labelledby="title desc">',
        '<title id="title">Deploy an event collector</title><desc id="desc">A backend sends HTTPS to public Caddy. Private API, Redpanda, consumer and PostgreSQL run on one Docker Compose host.</desc>',
        style,
        '<rect width="360" height="850" rx="12" fill="var(--paper)"/>',
        '<text x="20" y="32" class="heading">Deploy an event collector</text>',
        '<rect x="20" y="190" width="320" height="580" rx="12" class="card"/>',
        '<text x="34" y="213" class="step">ONE HOST</text>',
        '<rect x="30" y="326" width="300" height="432" rx="10" fill="none" stroke="var(--blue)" stroke-dasharray="5 4"/>',
        '<text x="40" y="346" class="small">Private services</text>',
        '<text x="40" y="363" class="small">No host ports</text>',
    ]
    cards = [
        (65, "Your backend", "Stable event UUIDs"),
        (230, "Caddy", "Public ports 80 / 443"),
        (385, "Go API", "Bearer key supplies tenant"),
        (485, "Redpanda", "Events + quarantine topics"),
        (585, "Go consumer", "Retry; commit SQL before offsets"),
        (685, "PostgreSQL", "One row per tenant + UUID"),
    ]
    for y, title, sub in cards:
        parts += [
            f'<rect x="40" y="{y}" width="280" height="64" rx="10" class="card"/>',
            f'<text x="54" y="{y+26}" class="heading">{title}</text>',
            f'<text x="54" y="{y+49}" class="small">{sub}</text>',
        ]
    for a, b, label in [
        (129, 230, "HTTPS"),
        (294, 385, "HTTP"),
        (449, 485, "publish"),
        (549, 585, "consume"),
        (649, 685, "batch INSERT"),
    ]:
        # Public entry passes through the host frame; other arrows stay in their boundary.
        label_y = 313 if label == "HTTP" else (a + b) // 2 + 5
        parts += [
            f'<path d="M180 {a+3}V{b-5}" class="arrow"/>',
            f'<path d="M175 {b-11}L180 {b-5}L185 {b-11}" class="arrow"/>',
            f'<text x="195" y="{label_y}" class="small">{label}</text>',
        ]
    parts += [
        '<text x="20" y="797" class="small">One host does not protect against disk loss.</text>',
        '<text x="20" y="824" class="small">Source 213e4cc · full deployment diagram linked</text></svg>',
    ]
    return "\n".join(parts)


if __name__ == "__main__":
    output = Path(__file__).resolve().parents[1] / "assets"
    (output / "delivery-mobile.svg").write_text(SVG + "\n")
    (output / "deployment-mobile.svg").write_text(deployment_guide() + "\n")
