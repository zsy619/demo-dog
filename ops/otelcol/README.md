# OpenTelemetry Collector Integration

demo-dog speaks the OTLP protocol natively. The OTel collector already
ships telemetry in OTLP form, so the integration is a single exporter
configuration. No translation layer, no sidecar, no agent.

## TL;DR

```bash
# 1. Start demo-dog collector on port 8088
dog-collector --api-keys=demo:$6rounds

# 2. Drop ops/otelcol/dog-collector.yaml into your otelcol install
#    (path varies; commonly /etc/otelcol/config.yaml)
cp ops/otelcol/dog-collector.yaml /etc/otelcol/config.yaml
systemctl restart otelcol

# 3. Set the API key
export DOG_API_KEY="demo:$6rounds"

# Done. OTel agents -> otelcol -> demo-dog.
```

## What the config does

The included `dog-collector.yaml` defines:

* `otlp` receiver on standard ports (4317 gRPC + 4318 HTTP)
* `batch` processor with 1s flush (default otelcol behavior; demo-dog
  doesn’t need batching for correctness, only for CPU efficiency)
* `memory_limiter` to prevent queue ballooning when demo-dog is down
* `attributes/keep_service` so we only ingest the `service.name`
  attribute (others are stripped)
* `otlphttp/dog` exporter pointing at demo-dog
  * `/v1/traces` (traces)
  * `/v1/metrics` (metrics)
  * `/v1/logs` (logs)
* `prometheusremotewrite/dog` exporter for long-term metrics retention
  on `/api/v1/write`

## Why two exporters?

Traces and logs are OTLP-only. Metrics can go through either OTLP/HTTP
or Prom Remote Write. Use:

* **OTLP/HTTP for metrics** when you need histograms with original bucket
  boundaries
* **Prom Remote Write for metrics** when you want Prometheus-compat
  storage, sample-rate control, or are migrating from a vanilla
  Prometheus server

You can enable both. demo-dog deduplicates on `(tenant, service, name,
labels, timestamp)`.

## Embedding demo-dog as a library

If you maintain a custom OTel collector fork and want to embed demo-dog
directly without HTTP, use `sdk/otlp-go`:

```go
import "github.com/zsy619/demo-dog/sdk/otlp-go"

// Inside your processor:
func (p *Processor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
    env := otlpgo.Envelope{
        TenantID: "acme",
        Service:  extractService(td),
    }
    for _, rs := range td.ResourceSpans().All() {
        for _, ss := range rs.ScopeSpans().All() {
            for _, span := range ss.Spans().All() {
                env.Spans = append(env.Spans, otlpgo.Span{
                    TraceID: span.TraceID().String(),
                    SpanID:  span.SpanID().String(),
                    Name:    span.Name(),
                    Start:   span.StartTimestamp().AsTime(),
                    End:     span.EndTimestamp().AsTime(),
                    Status:  otelStatusToString(span.Status().Code()),
                })
            }
        }
    }
    bundle := env.ToBundle()
    return p.ingest.Submit(ctx, bundle)
}
```

`examples/otel-bridge/` ships a runnable end-to-end demo:

```bash
cd sdk/otlp-go && go run ./examples/otel-bridge
```

## Migration from OTel collector (no demo-dog) to demo-dog

You don’t need to change your agents. The OTel SDKs already export OTLP.
The only change is the **exporter** in your otelcol config — point it
at demo-dog instead of wherever it currently sends.

If you previously exported to multiple backends (Jaeger for traces,
Loki for logs, Prometheus for metrics), you can keep all three and
**add** demo-dog as a fourth. demo-dog becomes the unified query plane.

## Compatibility matrix

| OTel signal | Protocol | demo-dog endpoint | Notes |
|---|---|---|---|
| Traces | OTLP/HTTP | `POST /v1/traces` | Full OTLP/JSON, scopes preserved |
| Metrics (gauge) | OTLP/HTTP | `POST /v1/metrics` | Sum/Gauge/Histogram |
| Metrics (counter) | OTLP/HTTP | `POST /v1/metrics` | Renamed `<m>_total` automatically |
| Metrics (histogram) | OTLP/HTTP | `POST /v1/metrics` | Buckets stored verbatim |
| Metrics (any) | Prom Remote Write | `POST /api/v1/write` | Snappy + protobuf + gzip |
| Logs | OTLP/HTTP | `POST /v1/logs` | Severity preserved |
| All | OTLP/gRPC | not supported | OTLP/HTTP only (no gRPC dep) |

## gRPC: why not?

demo-dog intentionally avoids gRPC. Reasons:

1. `google.golang.org/grpc` brings 12+ transitive dependencies and a
   generated `.pb.go` that has to be vendored. We stay stdlib-only.
2. OTLP/HTTP is the canonical protocol since OTel SDK 1.4+. All major
   language SDKs support it as a first-class export target.
3. gRPC adds zero observability value over HTTP/1.1 + JSON for our
   payload size (typically <100 KB per batch).

If a future user demands gRPC, the path is:

* add `golang.org/x/net/http2` + `google.golang.org/grpc` + `google.golang.org/protobuf`
* regenerate `.pb.go` from `sdk/otlp-proto/collector_trace_service.proto`
* mount a second listener on `:4317`

Roughly 200 lines of glue. Tracked as Round 33.
