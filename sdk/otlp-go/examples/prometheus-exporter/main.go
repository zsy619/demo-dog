// prometheus-exporter -- exposes the SDK buffer as a Prometheus
// /metrics scrape endpoint, AND also pushes to the DOG collector.
// Useful when you want both: a local scrape endpoint for Grafana and
// the central collector for service aggregation.
//
//   go run ./examples/prometheus-exporter
//   # scrape it:
//   curl http://localhost:8086/metrics
//   # or hit the demo endpoint:
//   curl http://localhost:8086/work
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
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

	sdk, err := otlp.New(endpoint,
		otlp.WithService("prometheus-exporter-demo"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithFlushInterval(2*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	// Periodic background metric emission so the /metrics endpoint has
	// something to show between requests.
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for range t.C {
			sdk.Gauge(context.Background(), "internal.queue_depth",
				float64(rand.Intn(100)),
				otlp.String("queue", "main"))
			sdk.Counter(context.Background(), "internal.heartbeat", 1)
		}
	}()

	// PrometheusCollector wires the SDK to a /metrics handler.
	col := otlp.NewPrometheusCollector(sdk, otlp.WithPrometheusPrefix("promexp_"))

	mux := http.NewServeMux()
	mux.Handle("/metrics", col.Handler())

	// Demo work endpoint: emits a few records per call.
	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		start := time.Now()
		sdk.Counter(ctx, "work.calls", 1)
		time.Sleep(time.Duration(20+rand.Intn(80)) * time.Millisecond)
		sdk.Histogram(ctx, "work.latency_ms",
			float64(time.Since(start).Milliseconds()),
			otlp.String("path", r.URL.Path),
		)
		// Flush immediately so a /metrics scrape right after /work sees it.
		sdk.ForceFlush(ctx)
		fmt.Fprintln(w, "ok")
	})

	log.Printf("listening on :8086 (metrics on /metrics, work on /work)")
	if err := http.ListenAndServe(":8086", mux); err != nil {
		log.Fatal(err)
	}
}
