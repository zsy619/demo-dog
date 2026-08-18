// loadtest — drive the SDK under burst pressure. Useful for verifying
// that the SDK doesn’t drop records when flushInterval is short and the
// queue is many thousands of items wide.
//
//   go run ./examples/loadtest
//
// Defaults: 50k logs + 50k metrics + 5k spans over 10 seconds. Override
// via -logs, -metrics, -spans, -duration.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

func main() {
	endpoint := flag.String("endpoint", "http://localhost:18080", "collector endpoint")
	svcName := flag.String("service", "loadtest", "service.name resource attribute")
	target := flag.Duration("duration", 10*time.Second, "how long to flood")
	logs := flag.Int("logs", 50000, "number of log records to emit")
	metrics := flag.Int("metrics", 50000, "number of metric records to emit")
	spans := flag.Int("spans", 5000, "number of spans to emit")
	workers := flag.Int("workers", 8, "producer goroutines")
	batch := flag.Int("batch", 2000, "per-flush record cap (stress the trim path)")
	flushInterval := flag.Duration("flush", 50*time.Millisecond, "flush interval")
	flag.Parse()

	sdk, err := otlp.New(*endpoint,
		otlp.WithService(*svcName),
		otlp.WithServiceVersion("loadtest"),
		otlp.WithFlushInterval(*flushInterval),
		otlp.WithMaxBatch(*batch),
	)
	if err != nil {
		log.Fatal(err)
	}

	deadline := time.Now().Add(*target)
	var sent int64
	var wg sync.WaitGroup

	// Producer: distribute load across `workers` goroutines.
	work := func() {
		defer wg.Done()
		ctx := context.Background()
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for time.Now().Before(deadline) {
			// Mix the three signal types in roughly equal shares.
			switch r.Intn(3) {
			case 0:
				if atomic.AddInt64(&sent, 1) <= int64(*logs) {
					sdk.Log(ctx, otlp.SeverityInfo, "loadtest log line",
						otlp.Int("seq", r.Int63()),
						otlp.String("user_id", fmt.Sprintf("u-%d", r.Intn(1000))),
					)
				}
			case 1:
				if atomic.AddInt64(&sent, 1) <= int64(*metrics) {
					sdk.Counter(ctx, "loadtest.counter", float64(r.Intn(5)),
						otlp.String("region", "us-east"),
					)
				}
			default:
				if atomic.AddInt64(&sent, 1) <= int64(*spans) {
					start := time.Now()
					time.Sleep(time.Duration(r.Intn(2)) * time.Millisecond)
					sdk.Record(ctx, "loadtest.span", start, nil)()
				}
			}
		}
	}

	log.Printf("flooding %s for %s ...", *svcName, *target)
	start := time.Now()
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go work()
	}
	wg.Wait()
	log.Printf("emitting done in %s", time.Since(start))

	if err := sdk.ForceFlush(context.Background()); err != nil {
		log.Printf("flush: %v", err)
	}
	if err := sdk.Shutdown(context.Background()); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Printf("done. service=%s sent=%d", *svcName, atomic.LoadInt64(&sent))
	os.Exit(0)
}
