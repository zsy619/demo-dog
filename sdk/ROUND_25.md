# Round 25 — Production ingest paths + multi-SDK

Round 25 closes the loop on real-world ingestion. After this round
the collector is reachable from every agent in the wild:

## Ingest endpoints (server-side)

| Path | Protocol | Wire format | Used by |
|---|---|---|---|
| `/api/ingest/otlp` | HTTP/1.1 POST | JSON, simplified | demo-dog SDKs, internal clients |
| `/api/ingest/otlp-json` | HTTP/1.1 POST | JSON, canonical OTLP | OTel SDKs exporting JSON |
| `/v1/logs` | HTTP/1.1 POST | OTLP/JSON | OTel collector → demo-dog |
| `/v1/metrics` | HTTP/1.1 POST | OTLP/JSON | OTel collector → demo-dog |
| `/v1/traces` | HTTP/1.1 POST | OTLP/JSON | OTel collector → demo-dog |
| `/api/v1/write` | HTTP/1.1 POST | snappy+protobuf | Prometheus agents |
| `/api/prom/write` | HTTP/1.1 POST | snappy+protobuf | alias of `/api/v1/write` |
| (gRPC, planned) | HTTP/2 | OTLP/protobuf | future round |

## SDKs

Four SDKs share the same wire contract:

| Language | Path | Stdlib-only |
|---|---|---|
| Go | `sdk/otlp-go/` | yes |
| Python | `sdk/otlp-python/` | yes |
| Node | `sdk/otlp-node/` | yes |
| Java | `sdk/otlp-java/` | yes (JDK 11+) |

Each SDK exposes `log`, `counter`, `gauge`, `histogram`, `span`,
plus a `flush()` / `close()` boundary and a bounded in-memory
buffer so backpressure cannot OOM the host process.

## Wire reference

The collector accepts two flavours:

* **Simplified** — `resource_attrs: {map}`, top-level `logs` /
  `metrics` / `spans`. Easier to hand-craft, used by demo-dog
  SDKs.
* **Canonical OTLP** — `resource: {attributes: [KeyValue]}`, with
  `scope_logs`, `scope_metrics`, `scope_spans` nested envelopes.
  Round 25 implements the JSON decoder; a future round adds the
  protobuf decoder for gRPC parity.

The collector normalises both flavours into the same internal
shape before they hit the store. Tenant attribution follows the
same priority chain: auth-bound key → body `tenant_id` → query
`?tenant=` (admin impersonation only).

## Prom Remote Write

The Prometheus wire is documented at
`https://prometheus.io/docs/concepts/remote_write_spec/`. We
implement the HTTP+protobuf subset used by every Prometheus agent:

* `Content-Encoding: snappy` (framed format) — primary
* `Content-Encoding: gzip` — fall-back for clients that prefer it
* `Content-Encoding: identity` — for tests / curl

Labels are flattened to attributes; `__name__` becomes the metric
name and `service` / `service_name` becomes the service. Everything
else is preserved.
