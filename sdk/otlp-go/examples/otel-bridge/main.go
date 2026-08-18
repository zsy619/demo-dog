// otel-bridge -- the inverse of T1.1.
//
// Shows how to use this SDK to scrape / ingest from a service that
// already speaks OTLP via the go.opentelemetry.io/otel SDK. We do NOT
// add a real OTel dependency; instead we simulate an OTel-shaped
// payload by building it locally and sending it through the SDK.
//
// In a real deployment, point your otel-collector / otel-sidecar at
// the SDK URL (using the standard envelope exporter from T1.1) and the
// SDK will forward everything to the DOG collector.
//
//   go run ./examples/otel-bridge
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	// Two SDKs: the standard envelope one (for OTel-shaped payloads) and
	// the simplified one (for the DOG-specific ingestion).
	sdkBridge, err := otlp.New(endpoint,
		otlp.WithService("otel-bridge"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithFlushInterval(2*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdkBridge.Shutdown(context.Background())

	// Simulate an OTel-ingested payload: a Service A produces a span
	// with a parent that Service B inherits. Both arrive at the bridge.
	ctx, end := sdkBridge.Trace(context.Background(), "service-a.handle")
	start := time.Now()
	time.Sleep(50 * time.Millisecond)

	// Pretend a sub-call into OTel-instrumented service B happens here.
	sdkBridge.Log(ctx, otlp.SeverityInfo, "calling service-b")
	end(nil)

	// Render the data using the OTel envelope exporter so downstream
	// systems that expect the OTel wire format stay happy.
	env := otlp.NewOTelExporter(endpoint + "/api/ingest/otlp-json")
	if err != nil {
		return
	}
	req := otlp.Request{
		ResourceAttrs: map[string]string{"service.name": "service-b"},
		Spans: []otlp.SpanRecord{{
			TraceID:    "00000000000000000000000000000099",
			SpanID:     "0000000000000099",
			ParentID:   "0000000000000001",
			Name:       "service-b.do",
			StartTime:  start,
			DurationMs: time.Since(start).Milliseconds(),
			Status:     otlp.StatusOK,
		}},
	}
	if _, err := env.Export(context.Background(), req); err != nil {
		log.Printf("bridge export: %v", err)
	} else {
		fmt.Println("bridged 1 OTel-shaped span to collector")
	}

	// Flush the simplified SDK side too.
	_ = sdkBridge.ForceFlush(context.Background())
	fmt.Println("bridge demo complete")
}
