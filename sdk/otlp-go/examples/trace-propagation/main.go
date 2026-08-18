// trace-propagation -- two cooperating services that propagate W3C
// trace context across HTTP. The downstream service extracts the
// incoming traceparent and continues the trace.
//
//   go run ./examples/trace-propagation
//   # in another terminal:
//   curl http://localhost:8084/work
//
// What you see in the DOG UI:
//   - service = trace-prop-a (one root trace)
//   - service = trace-prop-b (sub-span that shares the trace_id with a)
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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

	// Both services share one SDK because they run in the same process.
	// In a real deployment they would be separate binaries with their own
	// service.name.
	sdk, err := otlp.New(endpoint,
		otlp.WithService("trace-prop-a+b"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithFlushInterval(500*time.Millisecond),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	prop := otlp.NewPropagator()
	mux := http.NewServeMux()

	// Service A: receives a request, starts a trace, calls B.
	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		ctx, end := sdk.Trace(r.Context(), "service-a.handle")
		defer end(nil)

		sdk.Log(ctx, otlp.SeverityInfo, "service a received work")
		sdk.Counter(ctx, "service_a.requests", 1,
			otlp.String("path", r.URL.Path),
		)

		// Call service B with trace context injected.
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:8085/down", nil)
		prop.InjectHTTPHeader(ctx, req)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			sdk.Log(ctx, otlp.SeverityError, "call to b failed",
				otlp.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		io.Copy(w, resp.Body)
	})

	// Service B: extracts trace context, continues the trace.
	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) {
		tc := prop.ExtractHTTPHeader(r)
		var ctx context.Context
		if tc != nil {
			ctx = prop.WithTraceContext(r.Context(), tc)
			sdk.Log(context.Background(), otlp.SeverityInfo,
				"service b received trace",
				otlp.String("trace_id", tc.TraceID),
				otlp.String("parent_id", tc.SpanID),
			)
		} else {
			ctx = r.Context()
			sdk.Log(ctx, otlp.SeverityWarn, "no trace context on incoming request")
		}

		start := time.Now()
		sdk.Record(ctx, "service-b.handle", start, nil)()
		sdk.Log(ctx, otlp.SeverityInfo, "service b done")

		// Echo back the trace_id so curl can confirm it propagated.
		if tc != nil {
			fmt.Fprintf(w, "trace_id=%s\n", tc.TraceID)
			return
		}
		w.Write([]byte("no traceparent received\n"))
	})

	go func() {
		log.Printf("service B listening on :8085")
		if err := http.ListenAndServe(":8085", mux); !errors.Is(err, http.ErrServerClosed) {
			log.Printf("b: %v", err)
		}
	}()

	log.Printf("service A listening on :8084, exporting to %s", endpoint)
	if err := http.ListenAndServe(":8084", mux); err != nil {
		log.Fatal(err)
	}
}
