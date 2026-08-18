// worker-loop — a background worker that emits periodic metrics and
// traces a recurring job. Demonstrates how the SDK plays with long-running
// goroutines that aren’t request-scoped.
//
//   go run ./examples/worker-loop
//
// What you see in DOG:
//   - counter "worker.jobs" incrementing once per tick
//   - histogram "worker.duration_ms" with one sample per tick
//   - gauge "queue.depth" updated from a fake source
//   - one trace per tick (root span "worker.tick")
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	sdk, err := otlp.New(endpoint,
		otlp.WithService("worker-demo"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithFlushInterval(500*time.Millisecond),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go runWorker(ctx, &wg, sdk)
	wg.Add(1)
	go pollQueueDepth(ctx, &wg, sdk)

	fmt.Println("worker-loop demo running. ctrl-C to stop.")
	fmt.Println("watch /api/services in the DOG UI; service = worker-demo")
	wg.Wait()
}

func runWorker(ctx context.Context, wg *sync.WaitGroup, sdk *otlp.SDK) {
	defer wg.Done()
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			i++
			// A trace per tick — covers the entire job, including any
			// sub-steps the worker does.
			jctx, end := sdk.Trace(ctx, "worker.tick")
			start := time.Now()

			// Pretend the worker did three sub-tasks: fetch, process, ack.
			subStart := time.Now()
			time.Sleep(time.Duration(30+rand.Intn(40)) * time.Millisecond)
			sdk.Record(jctx, "worker.fetch", subStart, nil)(
				otlp.Int("iteration", int64(i)),
			)

			subStart = time.Now()
			time.Sleep(time.Duration(50+rand.Intn(80)) * time.Millisecond)
			sdk.Record(jctx, "worker.process", subStart, nil)(
				otlp.Int("iteration", int64(i)),
			)

			subStart = time.Now()
			time.Sleep(time.Duration(20+rand.Intn(20)) * time.Millisecond)
			sdk.Record(jctx, "worker.ack", subStart, nil)()

			durMs := time.Since(start).Milliseconds()
			sdk.Counter(jctx, "worker.jobs", 1,
				otlp.Int("iteration", int64(i)),
			)
			sdk.Histogram(jctx, "worker.duration_ms", float64(durMs),
				otlp.Int("iteration", int64(i)),
			)
			sdk.Log(jctx, otlp.SeverityInfo, "worker tick complete",
				otlp.Int("iteration", int64(i)),
				otlp.Int("duration_ms", durMs),
			)
			end(nil)
		}
	}
}

func pollQueueDepth(ctx context.Context, wg *sync.WaitGroup, sdk *otlp.SDK) {
	defer wg.Done()
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// Random walk: queue depth drifts up and down.
			depth := 100 + rand.Intn(50)
			sdk.Gauge(ctx, "queue.depth", float64(depth),
				otlp.String("queue", "main"),
			)
		}
	}
}
