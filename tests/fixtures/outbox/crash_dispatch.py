"""Test-only pause after a real HTTP ACK, before dispatch_once updates SQLite."""
import os
import time
from examples.checkout.ingest_client import Client
from examples.outbox.app import dispatch_once

class ParkAfterAck:
    def __init__(self):
        self.client = Client(os.environ["API_URL"], os.environ["INGESTKIT_API_KEY"],
                             ca_file=os.environ["INGESTKIT_CA_FILE"])
    def send_batch(self, events):
        ids = self.client.send_batch(events)
        print("broker-acknowledged", flush=True)
        while True:
            time.sleep(1)
        return ids

dispatch_once(os.environ["OUTBOX_DATABASE"], ParkAfterAck())
