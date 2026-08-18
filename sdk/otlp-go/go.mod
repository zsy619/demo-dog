module github.com/zsy619/demo-dog/sdk/otlp-go

// otlp-go is a pure-Go client SDK that targets the DOG collector ingest
// API. It speaks the same JSON-simplified OTLP envelope the backend accepts
// at /api/ingest/otlp-json (Content-Type: application/json+otlp) plus a
// permissive fallback at /api/ingest/otlp for plain JSON.
//
//   feature parity with the backend ingest contract:
//   - resource_attrs (map[string]string) required
//   - logs / metrics / spans all optional
//   - severity strings: TRACE, DEBUG, INFO, WARN, ERROR, FATAL
//   - span status: ok | error | unset
//   - metric type:   gauge | counter | histogram
//
// The SDK is built on the standard library only — no external deps — so it
// can be vendored into a Go service or used as a library import in a few
// lines:
//
//   sdk, _ := otlp.New("http://localhost:18080",
//       otlp.WithService("checkout"),
//       otlp.WithServiceVersion("v1.2.3"),
//   )
//   defer sdk.Shutdown(ctx)
//
//   sdk.Log(ctx, otlp.LogInfo, "order placed",
//       otlp.String("user_id", "u-42"))
//   sdk.Counter(ctx, "orders.placed", 1)
//   sdk.Record(ctx, "GET /checkout", 78*time.Millisecond, nil)

go 1.22
