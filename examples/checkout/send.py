#!/usr/bin/env python3
"""Send the fixed checkout fixture; preserve event IDs across uncertain retries."""
import json
import os
from pathlib import Path
import sys
import time
import urllib.error
import urllib.request

events = json.loads(Path(__file__).with_name("events.json").read_text())
body = json.dumps(events).encode()
url = os.environ.get("API_URL", "http://localhost:8080").rstrip("/") + "/v1/events/batch"
key = os.environ.get("INGESTKIT_API_KEY", "local-demo-key")
for attempt in range(3):
    request = urllib.request.Request(url, data=body, headers={
        "Content-Type": "application/json", "Authorization": "Bearer " + key})
    try:
        with urllib.request.urlopen(request, timeout=8) as response:
            if response.status != 202:
                raise RuntimeError("expected broker acknowledgement")
            ack = json.load(response)
            if ack["event_ids"] != [event["event_id"] for event in events["events"]]:
                raise RuntimeError("acknowledgement IDs differ")
            print(json.dumps(ack))
            sys.exit(0)
    except urllib.error.HTTPError as error:
        if error.code not in (429, 503):
            sys.exit("HTTP " + str(error.code))
    except (urllib.error.URLError, TimeoutError):
        pass
    if attempt < 2:
        time.sleep(0.1 * (2 ** attempt))
sys.exit("delivery uncertain; retry with the same event IDs")
