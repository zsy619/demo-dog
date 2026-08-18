// gin -- middleware example for the Gin web framework.
//
// We deliberately do NOT add github.com/gin-gonic/gin as a dependency.
// Instead this example uses net/http (the stdlib serves the same purpose
// for a demo) and the file ends with a fully formed Gin-shaped snippet
// that you can copy into a real Gin app. The non-trivial part -- the
// otlp middleware -- is identical in either case.
//
//   go run ./examples/gin
//   # hit it:
//   curl http://localhost:8087/hello
//   curl http://localhost:8087/error
//   curl http://localhost:8087/metrics   # prometheus scrape
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
		otlp.WithService("gin-demo"),
		otlp.WithServiceVersion("0.1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello from the gin-demo\n"))
	})
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	col := otlp.NewPrometheusCollector(sdk, otlp.WithPrometheusPrefix("gin_"))
	mux.Handle("/metrics", col.Handler())

	handler := ginStyleMiddleware(sdk, mux)
	log.Printf("listening on :8087, exporting to %s", endpoint)
	log.Fatal(http.ListenAndServe(":8087", handler))
}

func ginStyleMiddleware(sdk *otlp.SDK, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, end := sdk.Trace(r.Context(), r.Method+" "+r.URL.Path)
		defer end(nil)
		start := time.Now()

		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		durMs := time.Since(start).Milliseconds()
		attrs := []otlp.KV{
			otlp.String("http.method", r.Method),
			otlp.String("http.path", r.URL.Path),
			otlp.Int("http.status", int64(rw.status)),
			otlp.Int("http.duration_ms", durMs),
		}
		sdk.Counter(ctx, "http.requests", 1, attrs...)
		sdk.Histogram(ctx, "http.duration_ms", float64(durMs), attrs...)

		sev := otlp.SeverityInfo
		switch {
		case rw.status >= 500:
			sev = otlp.SeverityError
		case rw.status >= 400:
			sev = otlp.SeverityWarn
		}
		sdk.Log(ctx, sev,
			r.Method+" "+r.URL.Path+" -> "+strconv.Itoa(rw.status),
			attrs...,
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// To wire this into a real Gin app, replace ginStyleMiddleware with:
//
//   import "github.com/gin-gonic/gin"
//
//   func OtlpMiddleware(sdk *otlp.SDK) gin.HandlerFunc {
//       return func(c *gin.Context) {
//           ctx, end := sdk.Trace(c.Request.Context(),
//               c.Request.Method+" "+c.FullPath())
//           defer end(nil)
//           start := time.Now()
//           c.Next()
//           durMs := time.Since(start).Milliseconds()
//           status := c.Writer.Status()
//           attrs := []otlp.KV{
//               otlp.String("http.method", c.Request.Method),
//               otlp.String("http.path", c.FullPath()),
//               otlp.Int("http.status", int64(status)),
//               otlp.Int("http.duration_ms", durMs),
//           }
//           sdk.Counter(ctx, "http.requests", 1, attrs...)
//           sdk.Histogram(ctx, "http.duration_ms", float64(durMs), attrs...)
//       }
//   }
//
//   r := gin.Default()
//   r.Use(OtlpMiddleware(sdk))
//   r.GET("/hello", func(c *gin.Context) { c.String(200, "hi") })
//   r.Run(":8087")
