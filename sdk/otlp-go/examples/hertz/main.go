// Hertz framework middleware example.
//
// Hertz (https://www.cloudwego.io/zh/hertz/) is ByteDance open sourced
// high-performance Golang HTTP framework. This example shows how to
// write a middleware that wraps every request in a Trace, logs the
// response, and emits per-route metrics.
//
//   go run ./examples/hertz
//   # then hit it with:
//   curl http://localhost:8088/hello
//   curl http://localhost:8088/users/42
//   curl http://localhost:8088/metrics    # Prometheus scrape
//
// To run this example you need to add Hertz to your module:
//   go get github.com/cloudwego/hertz
package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"strconv"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	sdk, err := otlp.New(endpoint,
		otlp.WithService("hertz-demo"),
		otlp.WithServiceVersion("0.1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	// Native Hertz middleware.
	h := server.Default(server.WithHostPorts(":8088"))
	h.Use(OtlpMiddleware(sdk))

	h.GET("/hello", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, map[string]string{"msg": "hello, dog!"})
	})
	h.GET("/users/:id", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, map[string]string{
			"id":   ctx.Param("id"),
			"path": string(ctx.Request.URI().Path()),
		})
	})
	h.GET("/error", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusInternalServerError, map[string]string{"err": "boom"})
	})

	// Expose /metrics alongside the framework so it can be scraped by
	// Prometheus without coupling it to a particular route.
	col := otlp.NewPrometheusCollector(sdk, otlp.WithPrometheusPrefix("hertz_"))
	h.GET("/metrics", func(c context.Context, ctx *app.RequestContext) {
		// /metrics scrapes the live buffer BEFORE flushing. The SDK's
		// prometheus exporter is a snapshot of the in-flight buffer, not
		// a cumulative counter.
		body, err := col.Render()
		if err != nil {
			ctx.JSON(consts.StatusInternalServerError, map[string]string{"err": err.Error()})
			return
		}
		ctx.Response.Header.Set("Content-Type", "text/plain; version=0.0.4")
		ctx.Response.SetBodyStream(bytes.NewReader(body), len(body))
	})

	log.Printf("hertz demo listening on :8088, exporting to %s", endpoint)

	// Background heartbeat so the /metrics endpoint always has
	// something to scrape, even between user requests.
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			sdk.Gauge(context.Background(), "internal.queue_depth",
				float64(time.Now().UnixNano()%100),
				otlp.String("queue", "main"))
			sdk.Counter(context.Background(), "internal.heartbeat", 1)
		}
	}()

	h.Spin()
}

// OtlpMiddleware is the standard telemetry middleware for Hertz. It
// works with any *otlp.SDK and adds a Trace, a Counter, a Histogram and
// a Log per request. The middleware is plain Go -- copy it into your
// own project freely.
func OtlpMiddleware(sdk *otlp.SDK) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		start := time.Now()
		path := string(ctx.Request.URI().Path())
		method := string(ctx.Request.Method())

		// Track trace via the SDK. This returns a context that downstream
		// handlers can use (e.g. for sdk.Log(ctx, ...) in business code).
		trCtx, end := sdk.Trace(c, method+" "+path)

		// Forward to the next handler.
		ctx.Next(c)

		status := int32(ctx.Response.StatusCode())
		if status == 0 {
			status = consts.StatusOK // Hertz default if Next ran without writing
		}
		durMs := time.Since(start).Milliseconds()
		durFloat := float64(durMs)

		attrs := []otlp.KV{
			otlp.String("http.method", method),
			otlp.String("http.target", path),
			otlp.Int("http.status", int64(status)),
			otlp.String("http.scheme", string(ctx.Request.URI().Scheme())),
			otlp.String("http.host", string(ctx.Host())),
			otlp.String("http.user_agent", string(ctx.UserAgent())),
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
		sdk.Log(trCtx, sev, method+" "+path+" -> "+strconv.Itoa(int(status)), attrs...)

		// 4xx / 5xx makes the trace span error so the UI flags it.
		var endErr error
		if status >= 500 {
			endErr = errFromStatus(status)
		}
		end(endErr)
	}
}

// errFromStatus is a tiny helper so we don't pull errors just for one use.
func errFromStatus(status int32) error {
	return &httpError{status: int(status)}
}

type httpError struct{ status int }

func (e *httpError) Error() string {
	return "http " + strconv.Itoa(e.status)
}
