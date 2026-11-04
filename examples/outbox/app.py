"""One business transaction, one stored event, one bounded dispatcher."""
from contextlib import closing
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import sqlite3
from uuid import uuid4

from examples.checkout.ingest_client import IngestError

_SCHEMA = Path(__file__).with_name("schema.sql").read_text()

def _connect(database):
    path = Path(database)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    try:
        descriptor = os.open(path, os.O_CREAT | os.O_EXCL | os.O_RDWR, 0o600)
    except FileExistsError:
        pass
    else:
        os.close(descriptor)
    connection = sqlite3.connect(path, timeout=5)
    try:
        connection.execute("PRAGMA foreign_keys=ON")
        connection.execute("PRAGMA synchronous=FULL")
        connection.executescript(_SCHEMA)
        return connection
    except Exception:
        connection.close()
        raise

def create_order(database, order_id, amount_cents):
    if (not isinstance(order_id, str) or not order_id or "\0" in order_id
            or len(order_id.encode("utf-8")) > 128 or type(amount_cents) is not int
            or not 0 <= amount_cents <= 9223372036854775807):
        raise ValueError("invalid order")
    with closing(_connect(database)) as connection, connection:
        connection.execute("BEGIN IMMEDIATE")
        existing = connection.execute("SELECT amount_cents,event_id FROM orders WHERE order_id=?", (order_id,)).fetchone()
        if existing:
            if existing[0] != amount_cents:
                raise ValueError("order already exists with a different amount")
            return existing[1]
        event_id = str(uuid4())
        body = json.dumps({"event_id":event_id,"event_type":"order_completed",
                           "occurred_at":datetime.now(timezone.utc).isoformat(),
                           "payload":{"order_id":order_id,"amount_cents":amount_cents}},
                          ensure_ascii=False, allow_nan=False, separators=(",", ":"))
        connection.execute("INSERT INTO orders(order_id,amount_cents,event_id) VALUES(?,?,?)",
                           (order_id, amount_cents, event_id))
        connection.execute("INSERT INTO outbox(event_id,order_id,body) VALUES(?,?,?)",
                           (event_id, order_id, body))
        return event_id

def status(database):
    with closing(_connect(database)) as connection:
        counts = {"pending":0,"acknowledged":0,"rejected":0}
        counts.update(connection.execute("SELECT state,count(*) FROM outbox GROUP BY state"))
        counts["issues"] = [{"event_id":row[0],"attempts":row[1],"error":row[2]} for row in
                            connection.execute("SELECT event_id,attempts,last_error FROM outbox WHERE last_error IS NOT NULL ORDER BY rowid LIMIT 100")]
        return counts

def dispatch_once(database, client, batch_size=100):
    if type(batch_size) is not int or not 1 <= batch_size <= 100:
        raise ValueError("batch_size must be 1..100")
    with closing(_connect(database)) as connection:
        selected = connection.execute("SELECT event_id,body FROM outbox WHERE state='pending' ORDER BY rowid LIMIT ?", (batch_size,)).fetchall()
    result = {"attempted":len(selected),"acknowledged":0,"rejected":0,"pending":0,"error":None}
    if selected:
        ids = [row[0] for row in selected]
        state, code = "acknowledged", None
        try:
            acknowledgement = client.send_batch([json.loads(row[1]) for row in selected])
            if acknowledgement != ids:
                raise IngestError("invalid acknowledgement", retryable=True)
        except IngestError as error:
            # Authentication/configuration problems retain work for a repaired
            # caller. Only known bad-input/size rejections are parked as rejected.
            http_status = error.status if type(error.status) is int and 100 <= error.status <= 599 else None
            state = "rejected" if http_status in (400, 413) else "pending"
            code = "http_" + str(http_status) if http_status is not None else "delivery_uncertain"
        with closing(_connect(database)) as connection, connection:
            connection.executemany("UPDATE outbox SET state=?,attempts=attempts+1,last_error=?,acknowledged_at=? WHERE event_id=? AND state='pending'",
                                   [(state,code,datetime.now(timezone.utc).isoformat() if state == "acknowledged" else None,event_id) for event_id in ids])
        result["acknowledged"] = len(ids) if state == "acknowledged" else 0
        result["rejected"] = len(ids) if state == "rejected" else 0
        result["error"] = code
    result["pending"] = status(database)["pending"]
    return result
