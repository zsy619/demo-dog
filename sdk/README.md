# demo-dog SDKs

Four SDKs share the same wire contract:

| Language | Path | Module | Depends on |
|---|---|---|---|
| Go | `otlp-go/` | `github.com/zsy619/demo-dog/sdk/otlp-go` | stdlib only |
| Python | `otlp-python/` | `demo_dog` | stdlib only |
| Node | `otlp-node/` | `@demo-dog/sdk` | stdlib only |
| Java | `otlp-java/` | `io.demodog.sdk` | JDK 11+ |

All four expose the same surface:

```
client.log(severity, body, attributes?)
client.counter(name, value, attributes?)
client.gauge(name, value, attributes?)
client.histogram(name, value, attributes?)
client.span(trace_id, span_id, name, duration_ms, status, parent_span_id?)
client.flush()      // explicit drain
client.close()      // AutoCloseable (Java), destructor (Python/Node)
```

The on-the-wire shape is identical to the simplified OTLP/JSON the
collector accepts at `POST /api/ingest/otlp`. SDKs are responsible
for batching; the collector does its own validation/normalisation.

## Wire format

```json
{
  "resource_attrs": { "service.name": "checkout", "service.version": "v1.2.3" },
  "tenant_id": "acme",
  "logs":    [{ "timestamp_ns": ..., "severity_text": "INFO", "body": "...", "attributes": {...} }],
  "metrics": [{ "timestamp": ..., "name": "orders.placed", "value": 1, "attributes": {...} }],
  "spans":   [{ "trace_id": "...", "span_id": "...", "parent_span_id": "...", "service": "...", "name": "...", "start_unix_nano": ..., "duration_ns": ..., "status": "ok" }]
}
```

`tenant_id` is optional; when omitted the server falls back to the
auth-bound tenant (key → tenant) or the global admin view.

## Cross-language E2E

`frontend/e2e/sdk-{go,python,node}.mjs` exercises each SDK
end-to-end. Run them in CI to prove the SDKs stay in sync with the
backend contract.
