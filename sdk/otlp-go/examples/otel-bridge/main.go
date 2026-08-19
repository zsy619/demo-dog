// Command otel-bridge demonstrates how to convert the OpenTelemetry
// Collector's wire format into the demo-dog ingest wire bundle.
//
// In production, otelcol talks OTLP/HTTP directly to demo-dog on
// /v1/{logs,metrics,traces} — no bridge is needed. This example is
// for users who:
//   1. Embed demo-dog as a library inside their own otelcol fork
//   2. Want to forward collector envelopes through the simplified
//      /api/ingest/otlp path
//   3. Need to test their pipeline against a mock OTLP source
//
// Run: go run ./examples/otel-bridge
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/zsy619/demo-dog/sdk/otlp-go"
)

func main() {
	env := otlpgo.Envelope{
		TenantID: "acme",
		Service:  "checkout",
		Logs: []otlpgo.LogRecord{{
			Timestamp: time.Now(),
			Severity:  otlpgo.SeverityInfo,
			Body:      "order placed",
			Attrs:     otlpgo.ResourceAttrs{"user_id": "u-42"},
		}},
		Metrics: []otlpgo.MetricPoint{{
			Timestamp: time.Now(),
			Name:      "orders.placed",
			Value:     1,
		}},
	}
	bundle := env.ToBundle()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(bundle); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
