import gc
import warnings
import contextlib
import http.server
import json
import os
from pathlib import Path
import socket
import ssl
import subprocess
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

from ingest_client import Client, IngestError

EVENT = {"event_id": "dc12060f-f3a6-4ed3-9ea9-2d396b260b41", "event_type": "order_completed",
         "occurred_at": "2026-11-04T00:00:00Z", "payload": {"private": "payload-canary"}}
REQUEST_ID = "f8c22d85-5e93-4cb5-a456-18b0f104bcca"

@contextlib.contextmanager
def server(actions, tls=None):
    seen = []
    class Handler(http.server.BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass
        def do_POST(self):
            body = self.rfile.read(int(self.headers["Content-Length"]))
            seen.append((self.path, body, self.headers.get("Authorization")))
            action = actions[min(len(seen)-1, len(actions)-1)]
            if action == "close":
                self.connection.shutdown(socket.SHUT_RDWR)
                return
            if action == "hang":
                time.sleep(.2)
                return
            status, ids = action
            self.send_response(status)
            self.send_header("X-Request-ID", REQUEST_ID)
            self.end_headers()
            try:
                self.wfile.write(json.dumps({"status":"accepted", "event_ids":ids,
                                            "message":"secret-key payload-canary postgres://credential"}).encode())
            except OSError:
                pass
    httpd = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    httpd.daemon_threads = True
    if tls:
        httpd.socket = tls.wrap_socket(httpd.socket, server_side=True)
    thread = threading.Thread(target=httpd.serve_forever, daemon=True)
    thread.start()
    try:
        yield ("https" if tls else "http") + "://127.0.0.1:" + str(httpd.server_port), seen
    finally:
        httpd.shutdown()
        httpd.server_close()
        thread.join()

class ClientTest(unittest.TestCase):
    def test_import_has_no_side_effects(self):
        result = subprocess.run(["python3", "-c", "import send; import ingest_client"],
                                cwd=Path(__file__).parent, env=dict(os.environ, EVENTS_FILE="/absent"),
                                capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, b"")

    def test_single_and_batch_exact_ack(self):
        for batch in (False, True):
            with server([(202, [EVENT["event_id"]])]) as (url, seen):
                client = Client(url, "secret-key")
                ids = client.send_batch([EVENT]) if batch else client.send(EVENT)
                self.assertEqual(ids, [EVENT["event_id"]])
                self.assertEqual(seen[0][0], "/v1/events/batch" if batch else "/v1/events")
                self.assertEqual(seen[0][2], "Bearer secret-key")

    def test_retry_reuses_bytes_and_is_bounded(self):
        for failure in (429, 503, "close", "hang"):
            action = failure if isinstance(failure, str) else (failure, [])
            with server([action, (202, [EVENT["event_id"]])]) as (url, seen):
                self.assertEqual(Client(url, "secret-key", timeout=.06).send(EVENT), [EVENT["event_id"]])
                self.assertEqual(len(seen), 2)
                self.assertEqual(seen[0][1], seen[1][1])
            with server([action]) as (url, seen):
                started = time.monotonic()
                with self.assertRaises(IngestError) as raised:
                    Client(url, "secret-key", timeout=.06, max_attempts=2).send(EVENT)
                self.assertTrue(raised.exception.retryable)
                self.assertEqual(len(seen), 2)
                self.assertLess(time.monotonic()-started, 2)
                for private in ("secret-key", "payload-canary", "postgres://"):
                    self.assertNotIn(private, str(raised.exception))

    def test_attempt_deadline_includes_dns_without_late_post(self):
        # A slow resolver must not hold up send() or publish after it returns.
        resolve = socket.getaddrinfo
        released, finished = threading.Event(), threading.Event()
        def delayed(*args, **kwargs):
            try:
                released.wait(.3)
                return resolve(*args, **kwargs)
            finally:
                finished.set()
        with server([(202, [EVENT["event_id"]])]) as (url, seen):
            url = url.replace("127.0.0.1", "localhost")
            with patch("socket.getaddrinfo", delayed):
                started = time.monotonic()
                try:
                    with self.assertRaises(IngestError) as raised:
                        Client(url, "secret-key", timeout=.03, max_attempts=1).send(EVENT)
                    elapsed = time.monotonic()-started
                    self.assertTrue(raised.exception.retryable)
                    self.assertLess(elapsed, .18)
                finally:
                    released.set()
                    self.assertTrue(finished.wait(1))
            self.assertEqual(seen, [])

    def test_attempt_deadline_is_shared_by_all_addresses(self):
        # socket.create_connection otherwise grants each address a fresh timeout.
        addresses = [(socket.AF_INET, socket.SOCK_STREAM, socket.IPPROTO_TCP, "",
                      ("192.0.2.1", 80))] * 4
        sockets = []
        def stalled_connect(sock, address):
            sockets.append(sock)
            threading.Event().wait(sock.gettimeout())
            raise socket.timeout()
        with patch("socket.getaddrinfo", return_value=addresses), \
                patch.object(socket.socket, "connect", stalled_connect):
            started = time.monotonic()
            with self.assertRaises(IngestError) as raised:
                Client("http://example.invalid", "secret-key", timeout=.04,
                       max_attempts=1).send(EVENT)
            self.assertTrue(raised.exception.retryable)
            self.assertLess(time.monotonic()-started, .12)
        self.assertTrue(sockets)
        self.assertTrue(all(sock.fileno() == -1 for sock in sockets))

    def test_unresolved_dns_work_is_bounded(self):
        # Timed-out DNS must not create one forever-blocked thread per send.
        release = threading.Event()
        lock = threading.Lock()
        active, peak = 0, 0
        def stalled(*args, **kwargs):
            nonlocal active, peak
            with lock:
                active += 1
                peak = max(peak, active)
            try:
                release.wait(.2)
                raise socket.gaierror()
            finally:
                with lock:
                    active -= 1
        with patch("socket.getaddrinfo", stalled):
            try:
                started = time.monotonic()
                for _ in range(12):
                    with self.assertRaises(IngestError) as raised:
                        Client("http://example.invalid", "secret-key", timeout=.01,
                               max_attempts=1).send(EVENT)
                    self.assertTrue(raised.exception.retryable)
                self.assertLess(time.monotonic()-started, .4)
                self.assertLessEqual(peak, 8)
            finally:
                release.set()
                deadline = time.monotonic()+1
                while active and time.monotonic() < deadline:
                    threading.Event().wait(.005)
                self.assertEqual(active, 0)

    def test_permanent_status_and_request_id(self):
        for status in (400, 401, 413):
            with server([(status, [])]) as (url, seen):
                with self.assertRaises(IngestError) as raised:
                    Client(url, "secret-key").send(EVENT)
                self.assertEqual(raised.exception.status, status)
                self.assertFalse(raised.exception.retryable)
                self.assertEqual(raised.exception.request_id, REQUEST_ID)
                self.assertEqual(len(seen), 1)

    def test_ack_mismatch_and_redirect_never_succeed(self):
        for action in ((202, ["wrong"]), (302, [])):
            with server([action]) as (url, seen):
                with self.assertRaises(IngestError):
                    Client(url, "secret-key", max_attempts=2).send(EVENT)
                self.assertLessEqual(len(seen), 2)

    def test_invalid_header_key_is_a_safe_configuration_error(self):
        with self.assertRaises(IngestError) as raised:
            Client("http://localhost", "private-key\nInjected: payload-canary")
        self.assertFalse(raised.exception.retryable)
        self.assertNotIn("private-key", str(raised.exception))

    def test_response_sockets_are_closed_on_http_rejection(self):
        with server([(401, [])]) as (url, _):
            with warnings.catch_warnings(record=True) as captured:
                warnings.simplefilter("always", ResourceWarning)
                with self.assertRaises(IngestError):
                    Client(url, "secret-key").send(EVENT)
                gc.collect()
                self.assertFalse([w for w in captured if issubclass(w.category, ResourceWarning)],
                                 "HTTP rejection must close its response/socket")

    def test_tls_requires_explicit_trust(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)
            conf = path/"openssl.conf"
            conf.write_text("[req]\nprompt=no\ndistinguished_name=dn\nx509_extensions=v3\n[dn]\nCN=localhost\n[v3]\nsubjectAltName=DNS:localhost,IP:127.0.0.1\nbasicConstraints=critical,CA:TRUE\n")
            cert, key = path/"cert.pem", path/"key.pem"
            subprocess.run(["openssl","req","-x509","-nodes","-newkey","rsa:2048","-days","1",
                            "-config",str(conf),"-keyout",str(key),"-out",str(cert)], check=True, capture_output=True)
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.load_cert_chain(str(cert), str(key))
            with server([(202,[EVENT["event_id"]])], context) as (url, _):
                with self.assertRaises(IngestError):
                    Client(url,"secret-key", max_attempts=1).send(EVENT)
                self.assertEqual(Client(url,"secret-key",ca_file=str(cert)).send(EVENT),[EVENT["event_id"]])

if __name__ == "__main__":
    unittest.main()
