"""demo_dog — Python SDK for the demo-dog collector.

Provides a thin OTLP-style client. Stdlib-only (no requests,
no urllib3) so it can be vendored into a constrained environment.

Usage:

    from demo_dog import Client
    client = Client(base_url="http://localhost:18080", api_key="...")
    client.log(service="checkout", body="hello", severity="INFO")
    client.counter(service="checkout", name="orders_placed", value=1)
    client.histogram(service="checkout", name="checkout_latency_ms",
                     value=42.5, attributes={"region": "us-east-1"})
    client.span(trace_id="abc", span_id="01", service="checkout",
                name="GET /x", duration_ms=120, status="ok")
    client.flush()
"""

import json
import os
import threading
import time
import urllib.request
import urllib.error
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional


DEFAULT_BASE_URL = os.environ.get("DOG_BASE_URL", "http://localhost:18080")


@dataclass
class Resource:
    """One logical application. Bound at client construction."""
    service: str
    version: str = ""
    attributes: Dict[str, str] = field(default_factory=dict)


class Client:
    """Buffered OTLP-style ingest client.

    Records are queued in-memory and flushed in batches on a
    background timer or when `flush()` is called explicitly. The
    buffer is bounded so an OOM cannot be triggered by upstream
    backpressure.
    """

    def __init__(
        self,
        base_url: str = DEFAULT_BASE_URL,
        api_key: Optional[str] = None,
        service: str = "python",
        version: str = "",
        tenant: Optional[str] = None,
        flush_interval: float = 1.0,
        max_buffer: int = 10_000,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key or os.environ.get("DOG_API_KEY", "")
        self.tenant = tenant
        self.resource = Resource(
            service=service, version=version,
            attributes={},
        )
        self.flush_interval = flush_interval
        self.max_buffer = max_buffer
        self._buf_logs: List[Dict[str, Any]] = []
        self._buf_metrics: List[Dict[str, Any]] = []
        self._buf_spans: List[Dict[str, Any]] = []
        self._lock = threading.Lock()
        self._stop = threading.Event()
        self._flusher = threading.Thread(
            target=self._flush_loop, daemon=True, name="dog-flusher"
        )
        self._flusher.start()

    # ---- public API ----

    def log(self, body: str, severity: str = "INFO", attributes=None):
        with self._lock:
            self._buf_logs.append({
                "timestamp_ns": int(time.time() * 1_000_000_000),
                "severity_text": severity,
                "body": body,
                "attributes": dict(attributes or {}),
            })
            self._maybe_drop()

    def counter(self, name: str, value: float, attributes=None):
        self._emit_metric(name, value, attributes)

    def gauge(self, name: str, value: float, attributes=None):
        self._emit_metric(name, value, attributes)

    def histogram(self, name: str, value: float, attributes=None):
        self._emit_metric(name, value, attributes)

    def _emit_metric(self, name, value, attributes):
        with self._lock:
            self._buf_metrics.append({
                "timestamp": int(time.time() * 1_000),
                "name": name,
                "value": float(value),
                "attributes": dict(attributes or {}),
            })
            self._maybe_drop()

    def span(
        self,
        trace_id: str,
        span_id: str,
        name: str,
        duration_ms: float,
        service: Optional[str] = None,
        status: str = "ok",
        parent_span_id: Optional[str] = None,
    ):
        with self._lock:
            now = time.time()
            self._buf_spans.append({
                "trace_id": trace_id,
                "span_id": span_id,
                "parent_span_id": parent_span_id or "",
                "service": service or self.resource.service,
                "name": name,
                "start_unix_nano": int(now * 1_000_000_000),
                "duration_ns": int(duration_ms * 1_000_000),
                "status": status,
            })
            # Cache the most recent trace + parent so the next flush
            # emits a W3C traceparent header. The span itself stays in
            # the buffer until flush(); callers can also call
            # .set_current(trace_id, span_id) directly.
            self._current_trace_id = trace_id
            self._current_span_id = parent_span_id or span_id
            self._maybe_drop()

    def set_current(self, trace_id: str, span_id: str) -> None:
        """Manually pin the current trace context for outbound calls."""
        with self._lock:
            self._current_trace_id = trace_id
            self._current_span_id = span_id

    def flush(self, timeout: float = 5.0) -> bool:
        """Synchronously ship the buffer. Returns True on success."""
        with self._lock:
            logs = self._buf_logs
            metrics = self._buf_metrics
            spans = self._buf_spans
            self._buf_logs = []
            self._buf_metrics = []
            self._buf_spans = []
        if not logs and not metrics and not spans:
            return True
        body = {
            "resource_attrs": {
                "service.name": self.resource.service,
                "service.version": self.resource.version,
            },
            "logs": logs,
            "metrics": metrics,
            "spans": spans,
        }
        if self.tenant:
            body["tenant_id"] = self.tenant
        try:
            self._post("/api/ingest/otlp", body, timeout=timeout)
            return True
        except Exception:
            return False

    def close(self):
        self._stop.set()
        self.flush()

    # ---- internals ----

    def _post(self, path, body, timeout=5.0):
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.api_key}",
        }
        if getattr(self, "_current_trace_id", None) and getattr(self, "_current_span_id", None):
            headers["traceparent"] = (
                f"00-{self._current_trace_id}-{self._current_span_id}-01"
            )
        req = urllib.request.Request(
            self.base_url + path,
            data=json.dumps(body).encode("utf-8"),
            method="POST",
            headers=headers,
        )
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status

    def _maybe_drop(self):
        # Hard ceiling on in-memory records. Drop the oldest if
        # the buffer would otherwise grow without bound.
        total = len(self._buf_logs) + len(self._buf_metrics) + len(self._buf_spans)
        if total > self.max_buffer:
            # Drop the oldest from each until we are under the cap.
            while total > self.max_buffer:
                if self._buf_logs:
                    self._buf_logs.pop(0); total -= 1
                if total <= self.max_buffer: break
                if self._buf_metrics:
                    self._buf_metrics.pop(0); total -= 1
                if total <= self.max_buffer: break
                if self._buf_spans:
                    self._buf_spans.pop(0); total -= 1

    def _flush_loop(self):
        while not self._stop.is_set():
            self._stop.wait(self.flush_interval)
            if self._stop.is_set():
                break
            self.flush()


def example():
    """Tiny example showing the SDK contract end-to-end."""
    import os
    base = os.environ.get("DOG_BASE_URL", "http://localhost:18080")
    key = os.environ.get("DOG_API_KEY", "admin")
    c = Client(base_url=base, api_key=key, service="demo-py")
    c.log("hello from python")
    c.counter("py.events", 1)
    c.histogram("py.duration_ms", 42.5)
    c.span("trace-1", "span-1", "do_work", 12.3, status="ok")
    c.close()


if __name__ == "__main__":
    example()
