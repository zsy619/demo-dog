// Histogram with explicit OTel buckets — the recommended way to report
// latency / size / duration metrics.
//
// When you configure WithHistogramBuckets(...), the SDK accumulates
// observations between flushes and emits one OTel histogram data point
// per series per flush (with per-bucket counts + sum/count/min/max).
// The backend uses those explicit bucket boundaries to compute true
// quantiles (p50/p95/p99) instead of approximating from a log-bucketed
// fallback.
//
//   go run ./examples/otel-histogram
//   curl http://localhost:18080/api/histogram/otel?service=hist-demo&name=http.duration_ms
//
// To run this example standalone you may need to fetch dependencies:
//   go mod tidy
package main

import (
	"context"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	// Standard OTel default buckets (in seconds). Tailor to your service.
	bounds := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, math.MaxFloat64}

	sdk, err := otlp.New(endpoint,
		otlp.WithService("hist-demo"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithHistogramBuckets(bounds),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	// Simulate 1000 requests with latencies around 30ms.
	go func() {
		t := time.NewTicker(20 * time.Millisecond)
		defer t.Stop()
		i := 0
		for range t.C {
			latency := 0.020 + float64(i%50)*0.001 // 20..70 ms
			sdk.Histogram(context.Background(), "http.duration_ms", latency,
				otlp.String("endpoint", "/checkout"),
				otlp.Int("status", http.StatusOK),
			)
			i++
			if i >= 1000 {
				return
			}
		}
	}()

	log.Printf("hist-demo reporting to %s — buckets=%v", endpoint, bounds)
	select {}
}
