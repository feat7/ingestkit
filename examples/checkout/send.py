#!/usr/bin/env python3
"""Run the checkout fixture; import ingest_client from backend application code."""
import json
import os
from pathlib import Path
import sys
if __package__:
    from .ingest_client import Client, IngestError
else:
    from ingest_client import Client, IngestError

def main():
    try:
        fixture = Path(os.environ.get("EVENTS_FILE", str(Path(__file__).with_name("events.json"))))
        events = json.loads(fixture.read_text())["events"]
        mode = os.environ.get("SEND_MODE", "batch")
        if mode not in ("single", "batch"):
            raise ValueError()
        client = Client(os.environ.get("API_URL", "http://localhost:8080"),
                        os.environ.get("INGESTKIT_API_KEY", "local-demo-key"),
                        ca_file=os.environ.get("INGESTKIT_CA_FILE"))
        for group in ([[event] for event in events] if mode == "single" else [events]):
            ids = client.send(group[0]) if mode == "single" else client.send_batch(group)
            print(json.dumps({"status":"accepted", "event_ids":ids}))
        return 0
    except IngestError as error:
        print(str(error), file=sys.stderr)
    except (OSError, ValueError, KeyError, TypeError):
        print("invalid fixture or caller configuration", file=sys.stderr)
    return 1

if __name__ == "__main__":
    sys.exit(main())
