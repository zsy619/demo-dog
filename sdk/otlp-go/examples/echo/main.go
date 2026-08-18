// Echo framework middleware example.
//
// Echo (https://echo.labstack.com/) is a popular Go web framework. This
// example shows how to write a one-line middleware that wraps every
// request in a Trace, logs the response, and emits per-route metrics.
//
//   go run ./examples/echo
//   # then hit it with:
//   curl http://localhost:8082/hello
//   curl http://localhost:8082/users/42
//
// To run this example you need to add Echo to your module:
//   go get github.com/labstack/echo/v4
//
// The example is wired to depend on Echo via a blank import so the SDK
// itself does not gain a new transitive dependency.
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
		otlp.WithService("echo-demo"),
		otlp.WithServiceVersion("0.1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	// The Echo middleware is small enough to inline. If you prefer the
	// framework-native `echo.MiddlewareFunc` shape, see the note at the
	// bottom of this file.
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello, dog!\n"))
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/users/"):]
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{" + strconv.Quote("id") + ":" + strconv.Quote(id) + "}\n"))
	})

	handler := echoStyleMiddleware(sdk, mux)

	log.Printf("echo-style demo listening on :8082, exporting to %s", endpoint)
	log.Fatal(http.ListenAndServe(":8082", handler))
}

// echoStyleMiddleware is a stdlib port of the Echo middleware. The
// Echo-specific variant is identical except the signature returns
// echo.MiddlewareFunc and calls c.Next() / extracts the status from
// the ResponseWriter:
//
//	return func(next echo.HandlerFunc) echo.HandlerFunc {
//	    return func(c echo.Context) error {
//	        req := c.Request()
//	        res := c.Response()
//	        ctx, end := sdk.Trace(req.Context(), req.Method+" "+req.URL.Path)
//	        defer end(c.Response().Status >= 500 ? errors.New(...) : nil)
//	        // ... counter / histogram
//	        return next(c)
//	    }
//	}
func echoStyleMiddleware(sdk *otlp.SDK, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, end := sdk.Trace(r.Context(), r.Method+" "+r.URL.Path)
		start := time.Now()

		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		durMs := time.Since(start).Milliseconds()
		attrs := []otlp.KV{
			otlp.String("http.method", r.Method),
			otlp.String("http.target", r.URL.Path),
			otlp.Int("http.status", int64(rw.status)),
		}
		sdk.Counter(ctx, "http.requests", 1, attrs...)
		sdk.Histogram(ctx, "http.duration_ms", float64(durMs), attrs...)

		sev := otlp.SeverityInfo
		if rw.status >= 500 {
			sev = otlp.SeverityError
		} else if rw.status >= 400 {
			sev = otlp.SeverityWarn
		}
		sdk.Log(ctx, sev, r.Method+" "+r.URL.Path+" -> "+strconv.Itoa(rw.status), attrs...)
		end(nil)
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

// To Go module — add to go.mod:
//
//   require github.com/labstack/echo/v4 v4.11.0
//
// then the echo-specific version becomes:
//
//   import "github.com/labstack/echo/v4"
//
//   func EchoMiddleware(sdk *otlp.SDK) echo.MiddlewareFunc {
//       return func(next echo.HandlerFunc) echo.HandlerFunc {
//           return func(c echo.Context) error {
//               ctx, end := sdk.Trace(c.Request().Context(),
//                   c.Request().Method+" "+c.Path())
//               start := time.Now()
//               err := next(c)
//               durMs := time.Since(start).Milliseconds()
//               status := c.Response().Status
//               attrs := []otlp.KV{
//                   otlp.String("http.method", c.Request().Method),
//                   otlp.String("http.target", c.Request().URL.Path),
//                   otlp.Int("http.status", int64(status)),
//               }
//               sdk.Counter(ctx, "http.requests", 1, attrs...)
//               sdk.Histogram(ctx, "http.duration_ms", float64(durMs), attrs...)
//               end(err)
//               return err
//           }
//       }
//   }
//
//   e := echo.New()
//   e.Use(EchoMiddleware(sdk))
//   e.GET("/hello", func(c echo.Context) error { return c.String(200, "hello") })
