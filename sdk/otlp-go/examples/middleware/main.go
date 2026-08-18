// Middleware example: a net/http middleware that wraps every request in a
// span and emits per-status metrics + a structured log line.
//
//   go run ./examples/middleware
//
// Then hit the demo server with curl to generate signals.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	sdk, err := otlp.New(endpoint,
		otlp.WithService("middleware-demo"),
		otlp.WithServiceVersion("0.1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello, dog!\n"))
	})
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	srv := &http.Server{
		Addr:    ":8081",
		Handler: otlpMiddleware(sdk, mux),
	}
	log.Printf("listening on :8081, exporting to %s", endpoint)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func otlpMiddleware(sdk *otlp.SDK, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, end := sdk.Trace(r.Context(), r.Method+" "+r.URL.Path)
		defer end(nil)

		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r.WithContext(ctx))

		durMs := time.Since(start).Milliseconds()
		attrs := []otlp.KV{
			otlp.String("http.method", r.Method),
			otlp.String("http.target", r.URL.Path),
			otlp.Int("http.status", int64(rw.status)),
		}

		sdk.Counter(ctx, "http.requests", 1, attrs...)
		sdk.Histogram(ctx, "http.duration_ms", float64(durMs), attrs...)
		sdk.Record(ctx, "middleware.handler", start, rw.err)(
			otlp.Int("http.status", int64(rw.status)))

		sev := otlp.SeverityInfo
		if rw.status >= 500 {
			sev = otlp.SeverityError
		} else if rw.status >= 400 {
			sev = otlp.SeverityWarn
		}
		sdk.Log(ctx, sev, r.Method+" "+r.URL.Path+" -> "+strconv.Itoa(rw.status),
			attrs...)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	err    error
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
