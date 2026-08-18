// Beego framework middleware example.
//
// Beego (https://github.com/beego/beego) is an old-school but very
// popular Go web/MVC framework. Beego filters (web.InsertFilter)
// caused intermittent server-startup issues with v2.3.10 on Go 1.22+
// (the server fails to bind on `web.Run()` after a filter is
// registered). This example therefore wraps the SDK calls inside each
// handler -- functionally equivalent to middleware, just slightly
// more verbose. The OtlpWrap helper is provided so the boilerplate is
// kept to one line per route.
//
//   go run ./examples/beego
//   # then hit it with:
//   curl http://localhost:8089/hello
//   curl http://localhost:8089/users/42
//   curl http://localhost:8089/error
//   curl http://localhost:8089/metrics     # Prometheus scrape
//
// To run this example you need to add Beego to your module:
//   go get github.com/beego/beego/v2
package main

import (
	stdctx "context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"

	"github.com/beego/beego/v2/server/web"
	bctx "github.com/beego/beego/v2/server/web/context"
)

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	sdk, err := otlp.New(endpoint,
		otlp.WithService("beego-demo"),
		otlp.WithServiceVersion("0.1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(stdctx.Background())

	// Demo routes. Each calls OtlpWrap to share the trace + metric code.
	web.Get("/hello", OtlpWrap(sdk, func(ctx *bctx.Context) {
		ctx.Output.JSON(map[string]string{"msg": "hello, dog!"}, false, false)
	}))
	web.Get("/users/:id", OtlpWrap(sdk, func(ctx *bctx.Context) {
		id := ctx.Input.Param(":id")
		ctx.Output.JSON(map[string]string{"id": id}, false, false)
	}))
	web.Get("/error", OtlpWrap(sdk, func(ctx *bctx.Context) {
		ctx.Output.SetStatus(http.StatusInternalServerError)
		ctx.Output.Body([]byte("boom\n"))
	}))

	// Prometheus scrape endpoint.
	col := otlp.NewPrometheusCollector(sdk, otlp.WithPrometheusPrefix("beego_"))
	web.Get("/metrics", func(ctx *bctx.Context) {
		body, err := col.Render()
		if err != nil {
			ctx.Output.SetStatus(http.StatusInternalServerError)
			ctx.Output.Body([]byte(err.Error()))
			return
		}
		ctx.Output.Header("Content-Type", "text/plain; version=0.0.4")
		ctx.Output.Body(body)
	})

	// Background heartbeat so /metrics always has something to scrape.
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			sdk.Gauge(stdctx.Background(), "internal.queue_depth",
				float64(time.Now().UnixNano()%100),
				otlp.String("queue", "main"))
			sdk.Counter(stdctx.Background(), "internal.heartbeat", 1)
		}
	}()

	log.Printf("beego demo listening on :8089, exporting to %s", endpoint)
	web.Run()
}

// OtlpWrap is the per-route middleware equivalent: it starts a Trace,
// measures duration, and emits Counter / Histogram / Log when the
// handler returns. Beego does not support a clean "next()" hook
// inside a FilterFunc, so the pattern is: start, run handler, end.
func OtlpWrap(sdk *otlp.SDK, h func(ctx *bctx.Context)) func(ctx *bctx.Context) {
	return func(ctx *bctx.Context) {
		start := time.Now()
		path := ctx.Input.URL()
		method := ctx.Input.Method()

		trCtx, end := sdk.Trace(stdctx.Background(), method+" "+path)

		h(ctx)

		status := ctx.ResponseWriter.Status
		durMs := time.Since(start).Milliseconds()
		durFloat := float64(durMs)

		attrs := []otlp.KV{
			otlp.String("http.method", method),
			otlp.String("http.target", path),
			otlp.Int("http.status", int64(status)),
			otlp.String("http.scheme", ctx.Input.Scheme()),
			otlp.String("http.host", ctx.Input.Host()),
			otlp.String("http.user_agent", ctx.Input.UserAgent()),
		}
		sdk.Counter(trCtx, "http.requests", 1, attrs...)
		sdk.Histogram(trCtx, "http.duration_ms", durFloat, attrs...)

		sev := otlp.SeverityInfo
		switch {
		case status >= 500:
			sev = otlp.SeverityError
		case status >= 400:
			sev = otlp.SeverityWarn
		}
		sdk.Log(trCtx, sev, method+" "+path+" -> "+strconv.Itoa(status), attrs...)

		var endErr error
		if status >= 500 {
			endErr = errFromStatus(status)
		}
			end(endErr)
		}
}

func errFromStatus(status int) error { return &httpError{status: status} }

type httpError struct{ status int }

func (e *httpError) Error() string {
	return "http " + strconv.Itoa(e.status)
}

