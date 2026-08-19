# demo-dog Python SDK

Stdlib-only OTLP-style client for the demo-dog collector.

```python
from demo_dog import Client
client = Client(
    base_url="http://localhost:18080",
    api_key="admin",
    service="checkout",
    tenant="acme",  # optional
)
client.log("order placed", severity="INFO", attributes={"user_id": "u-42"})
client.counter("orders.placed", 1)
client.histogram("checkout.duration_ms", 78.5)
client.span("trace-1", "span-1", "GET /checkout", 120, status="ok")
client.close()  # flushes remaining buffer
```

The SDK buffers records in-memory and flushes them in batches
every `flush_interval` seconds (default 1s). The buffer is bounded
by `max_buffer` (default 10000) so backpressure cannot OOM the
process.
