#!/usr/bin/env python3
"""Plain HTTP; reuse serialized event IDs on every uncertain retry."""
import json
import os
from pathlib import Path
import sys
import time
import urllib.error
import urllib.request

fixture = Path(os.environ.get("EVENTS_FILE", str(Path(__file__).with_name("events.json"))))
events = json.loads(fixture.read_text())["events"]
mode = os.environ.get("SEND_MODE", "batch")
if mode not in ("single", "batch"):
    sys.exit("SEND_MODE must be single or batch")
base = os.environ.get("API_URL", "http://localhost:8080").rstrip("/")
key = os.environ.get("INGESTKIT_API_KEY", "local-demo-key")
for group in ([[event] for event in events] if mode == "single" else [events]):
    body = json.dumps(group[0] if mode == "single" else {"events": group}).encode()
    url = base + ("/v1/events" if mode == "single" else "/v1/events/batch")
    for attempt in range(3):
        request = urllib.request.Request(url, data=body, headers={
            "Content-Type": "application/json", "Authorization": "Bearer " + key})
        try:
            with urllib.request.urlopen(request, timeout=8) as response:
                if response.status != 202:
                    raise RuntimeError("expected broker acknowledgement")
                ack = json.load(response)
                if ack["event_ids"] != [event["event_id"] for event in group]:
                    sys.exit("acknowledgement IDs differ; delivery uncertain")
                print(json.dumps(ack))
                break
        except urllib.error.HTTPError as error:
            if error.code not in (429, 503):
                sys.exit("HTTP " + str(error.code))
        except (urllib.error.URLError, TimeoutError):
            pass
        if attempt < 2:
            time.sleep(0.1 * (2 ** attempt))
    else:
        sys.exit("delivery uncertain; retry with the same event IDs")
