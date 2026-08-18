// Quickstart example.
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

	sdk, err := otlp.New(endpoint,
		otlp.WithService("quickstart"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithDeploymentEnvironment("demo"),
		otlp.WithFlushInterval(1*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := sdk.Shutdown(context.Background()); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		sdk.Log(ctx, otlp.SeverityInfo, "hello from quickstart",
			otlp.Int("iteration", int64(i)),
			otlp.String("channel", "demo"),
		)
		sdk.Counter(ctx, "quickstart.iterations", 1,
			otlp.String("loop", "main"))
		sdk.Gauge(ctx, "quickstart.queue_depth", float64(42-i))
	}

	ctx, end := sdk.Trace(ctx, "demo.do_work")
	start := time.Now()
	time.Sleep(120 * time.Millisecond)
	if err := sdk.ForceFlush(ctx); err != nil {
		log.Printf("force flush: %v", err)
	}
	end(nil)
	sdk.Record(ctx, "demo.do_work.charge", start, nil)(
		otlp.String("merchant", "acme"))

	fmt.Println("emitted 5 logs, 5 counters, 1 gauge, 1 trace, 1 span")
}
