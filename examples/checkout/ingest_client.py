"""Copyable backend HTTP client. The caller creates and retains each event ID."""
import http.client
import json
import math
import re
import socket
import ssl
import threading
import time
from urllib.parse import urlsplit

# DNS has no cancellable stdlib API. Only these bounded daemon workers may
# outlive a timed-out attempt; they never connect or receive event/key data.
_DNS_SLOTS = threading.BoundedSemaphore(8)

def _remaining(deadline):
    budget = deadline-time.monotonic()
    if budget <= 0:
        raise TimeoutError()
    return budget

def _resolve(host, port, deadline):
    if not _DNS_SLOTS.acquire(timeout=_remaining(deadline)):
        raise TimeoutError()
    result, done = [], threading.Event()
    def lookup():
        try:
            try:
                result.append(socket.getaddrinfo(host, port, type=socket.SOCK_STREAM))
            except (OSError, UnicodeError):
                result.append(None)
        finally:
            _DNS_SLOTS.release()
            done.set()
    try:
        threading.Thread(target=lookup, daemon=True).start()
    except RuntimeError:
        _DNS_SLOTS.release()
        raise OSError("resolver unavailable") from None
    if not done.wait(_remaining(deadline)):
        raise TimeoutError()
    _remaining(deadline)  # Discard results that arrived after the deadline.
    if result[0] is None:
        raise OSError("resolution failed")
    return result[0]

_UUID = re.compile(r"^[0-9a-fA-F]{8}-(?:[0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$")

class IngestError(Exception):
    def __init__(self, message, *, status=None, retryable=False, request_id=None):
        super().__init__(message)
        self.status, self.retryable = status, retryable
        self.request_id = request_id if isinstance(request_id, str) and _UUID.fullmatch(request_id) else None

class Client:
    def __init__(self, base_url, key, timeout=8, max_attempts=3, ca_file=None):
        try:
            url = urlsplit(base_url)
            if (url.scheme not in ("http", "https") or not url.hostname or url.username is not None
                    or url.password is not None or url.query or url.fragment or not isinstance(key, str)
                    or not key or any(ord(c)<33 or ord(c)>126 for c in key) or not math.isfinite(timeout) or timeout <= 0
                    or not isinstance(max_attempts, int) or not 1 <= max_attempts <= 10):
                raise ValueError()
            self._host, self._port, self._secure = url.hostname, url.port, url.scheme == "https"
            self._path, self._key = url.path.rstrip("/"), key
            self._timeout, self._attempts = timeout, max_attempts
            self._context = ssl.create_default_context(cafile=ca_file) if self._secure else None
        except (ValueError, TypeError, OSError):
            raise IngestError("invalid client configuration") from None

    def send(self, event):
        return self._send([event], False)

    def send_batch(self, events):
        return self._send(events, True)

    def _send(self, events, batch):
        try:
            ids = [event["event_id"] for event in events]
            if not ids or any(not isinstance(i, str) or not _UUID.fullmatch(i) for i in ids):
                raise ValueError()
            expected = [i.lower() for i in ids]
            body = json.dumps({"events": events} if batch else events[0],
                              ensure_ascii=False, allow_nan=False, separators=(",", ":")).encode()
        except (KeyError, TypeError, ValueError, IndexError, OverflowError):
            raise IngestError("event serialization or event ID is invalid") from None
        path = self._path + ("/v1/events/batch" if batch else "/v1/events")
        error = IngestError("delivery uncertain; retain the same event IDs", retryable=True)
        for attempt in range(self._attempts):
            try:
                return self._attempt(path, body, expected)
            except IngestError as failure:
                error = failure
                if not error.retryable:
                    raise error from None
            if attempt+1 < self._attempts:
                time.sleep(.1 * 2**attempt)
        raise error from None

    def _connect(self, deadline):
        port = self._port if self._port is not None else (443 if self._secure else 80)
        for family, kind, protocol, _, address in _resolve(self._host, port, deadline):
            sock = None
            try:
                budget = _remaining(deadline)
                sock = socket.socket(family, kind, protocol)
                sock.settimeout(budget)
                sock.connect(address)
                sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
                if self._secure:
                    sock.settimeout(_remaining(deadline))
                    sock = self._context.wrap_socket(sock, server_hostname=self._host)
                _remaining(deadline)
                return sock
            except ssl.SSLCertVerificationError:
                if sock:
                    sock.close()
                raise
            except OSError:
                if sock:
                    sock.close()
        raise OSError("connection failed")

    def _attempt(self, path, body, expected):
        deadline = time.monotonic()+self._timeout
        connection = (http.client.HTTPSConnection(self._host, self._port, timeout=self._timeout,
                                                  context=self._context) if self._secure else
                      http.client.HTTPConnection(self._host, self._port, timeout=self._timeout))
        timer = None
        response = None
        try:
            connection.sock = self._connect(deadline)
            sock = connection.sock
            def stop():
                try:
                    sock.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass
            timer = threading.Timer(_remaining(deadline), stop)
            timer.daemon = True
            timer.start()
            connection.request("POST", path, body=body, headers={"Content-Type":"application/json",
                                                               "Authorization":"Bearer " + self._key})
            response = connection.getresponse()
            request_id = response.getheader("X-Request-ID")
            status = response.status
            if status != 202:
                raise IngestError("HTTP " + str(status), status=status,
                                  retryable=status == 429 or status >= 500, request_id=request_id)
            raw = response.read(65537)
            try:
                ack = json.loads(raw) if len(raw) <= 65536 else None
            except (ValueError, UnicodeError):
                ack = None
            if not isinstance(ack, dict) or ack.get("event_ids") != expected or ack.get("status") != "accepted":
                raise IngestError("acknowledgement differs; delivery uncertain", status=202,
                                  retryable=True, request_id=request_id)
            _remaining(deadline)
            return expected
        except ssl.SSLCertVerificationError:
            raise IngestError("TLS certificate is not trusted") from None
        except (OSError, http.client.HTTPException):
            raise IngestError("connection failed; delivery uncertain", retryable=True) from None
        finally:
            if timer:
                timer.cancel()
            if response:
                response.close()
            connection.close()
