// sampler-debug -- demonstrates the SDK sampler hook.
//
// The program emits 1000 trace roots, then prints how many were sampled
// (and how many dropped), using both AlwaysOn and TraceIDRatioBased
// samplers. Useful for tuning sampling rates before pushing to prod.
//
//   go run ./examples/sampler-debug
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

	// 50% sampling to demonstrate the SDK dropping the other half.
	sdk, err := otlp.New(endpoint,
		otlp.WithService("sampler-debug"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithFlushInterval(100*time.Millisecond),
		otlp.WithSampler(otlp.NewTraceIDRatioBased(0.5)),
		otlp.WithErrorHandler(func(err error) {
			// Silently ignore so the demo output stays clean.
			_ = err
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	const total = 1000
	for i := 0; i < total; i++ {
		_, end := sdk.Trace(context.Background(), fmt.Sprintf("op-%d", i))
		sdk.Counter(context.Background(), "debug.iterations", 1)
		end(nil)
	}

	// Force a flush so the buffer drains before we read stats.
	_ = sdk.ForceFlush(context.Background())

	st := sdk.Stats()
	sampled := st.SpansEmitted
	skipped := st.SamplerSkipped
	fmt.Printf("total traces started: %d\n", total)
	fmt.Printf("spans emitted:        %d\n", sampled)
	fmt.Printf("sampler skipped:      %d\n", skipped)
	fmt.Printf("flush calls:          %d\n", st.FlushCalls)
	fmt.Printf("flush errors:         %d\n", st.FlushErrors)
	fmt.Printf("requeued logs:        %d\n", st.RequeuedLogs)
	fmt.Printf("expected ratio ~0.5; actual = %.2f\n", float64(sampled)/float64(total))
}
