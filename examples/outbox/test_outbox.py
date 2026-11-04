import json
from pathlib import Path
import sqlite3
import subprocess
import tempfile
import unittest

from examples.checkout.ingest_client import IngestError
from examples.outbox.app import create_order, dispatch_once, status

class Transport:
    def __init__(self, error=None, wrong=False):
        self.error, self.wrong, self.bodies = error, wrong, []
    def send_batch(self, events):
        self.bodies.append(json.dumps(events, separators=(",", ":")))
        if self.error:
            raise self.error
        return ["wrong"] if self.wrong else [event["event_id"] for event in events]

class OutboxTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.database = str(Path(self.temp.name)/"orders.db")
    def row(self, order_id="order-1"):
        with sqlite3.connect(self.database) as connection:
            return connection.execute("SELECT event_id,body,state,attempts,last_error FROM outbox WHERE order_id=?", (order_id,)).fetchone()
    def test_order_and_outbox_are_one_transaction(self):
        create_order(self.database,"seed",100)
        with sqlite3.connect(self.database) as connection:
            connection.execute("CREATE TRIGGER fail_outbox BEFORE INSERT ON outbox BEGIN SELECT RAISE(ABORT, 'test failure'); END;")
        with self.assertRaises(sqlite3.IntegrityError):
            create_order(self.database,"order-1",2500)
        with sqlite3.connect(self.database) as connection:
            self.assertEqual(connection.execute("SELECT count(*) FROM orders WHERE order_id='order-1'").fetchone()[0],0)
    def test_business_retry_keeps_id_and_bytes_across_processes(self):
        event_id = create_order(self.database,"order-1",2500)
        before = self.row()
        result = subprocess.check_output(["python3","-m","examples.outbox","create-order","--database",self.database,"--order-id","order-1","--amount-cents","2500"],text=True)
        self.assertEqual(json.loads(result)["event_id"],event_id)
        self.assertEqual(self.row(),before)
        with self.assertRaises(ValueError):
            create_order(self.database,"order-1",2600)
        self.assertEqual(self.row(),before)
    def test_transient_and_authentication_failures_stay_pending(self):
        for failure in (IngestError("payload-canary",retryable=True),IngestError("private-key",status=401),
                        IngestError("private-key",status="private-key")):
            create_order(self.database,"order-1",2500)
            transport = Transport(error=failure)
            summary = dispatch_once(self.database,transport)
            self.assertEqual(summary["attempted"],1)
            self.assertEqual(summary["acknowledged"],0)
            self.assertEqual(self.row()[2],"pending")
            self.assertNotIn("payload-canary",json.dumps(summary))
            self.assertNotIn("private-key",json.dumps(summary))
        success = Transport()
        summary = dispatch_once(self.database,success)
        self.assertEqual(summary["acknowledged"],1)
        self.assertEqual(self.row()[2],"acknowledged")
        self.assertEqual(transport.bodies[-1],success.bodies[0])
        self.assertEqual(dispatch_once(self.database,success)["attempted"],0)
    def test_permanent_rejection_is_visible(self):
        event_id=create_order(self.database,"order-1",2500)
        result=dispatch_once(self.database,Transport(error=IngestError("private payload",status=400)))
        self.assertEqual(result["rejected"],1)
        self.assertEqual(self.row()[2],"rejected")
        self.assertEqual(self.row()[4],"http_400")
        self.assertEqual(status(self.database)["rejected"],1)
        self.assertEqual(self.row()[0],event_id)
        self.assertEqual(dispatch_once(self.database,Transport())["attempted"],0)
    def test_dispatch_is_bounded_and_bad_ack_is_not_success(self):
        for i in range(3):
            create_order(self.database,"order-"+str(i),100)
        result=dispatch_once(self.database,Transport(wrong=True),batch_size=2)
        self.assertEqual(result["attempted"],2)
        self.assertEqual(result["acknowledged"],0)
        self.assertEqual(result["pending"],3)
        result=dispatch_once(self.database,Transport(),batch_size=2)
        self.assertEqual(result["acknowledged"],2)
        self.assertEqual(result["pending"],1)
        for invalid in (0,101):
            with self.assertRaises(ValueError):
                dispatch_once(self.database,Transport(),batch_size=invalid)

if __name__ == "__main__":
    unittest.main()
