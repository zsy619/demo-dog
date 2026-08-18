// db-tracing — wrap a sql.DB so every Query / Exec emits a span and a
// pair of metrics (calls + latency histogram) to the DOG collector.
//
// This example uses sqlmock as an in-memory stand-in for a real database
// driver. The wrapper does NOT add a dependency on sqlmock at the SDK
// level; we only use it here so the demo is self-contained.
//
//   go run ./examples/db-tracing
//
// Expected output in DOG:
//   - one service called "db-tracing-demo"
//   - SQL calls counted under db.calls{sql=…,status=…}
//   - SQL latency recorded under db.duration_ms{sql=…}
//   - one trace per call when run with a parent Trace
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

// otlpDB wraps a *sql.DB. All exported methods forward to the underlying
// driver, prefixed with a span + counter + histogram.
type otlpDB struct {
	inner *sql.DB
	sdk   *otlp.SDK
}

func wrap(sdk *otlp.SDK, inner *sql.DB) *otlpDB { return &otlpDB{inner: inner, sdk: sdk} }

func (d *otlpDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.inner.QueryContext(ctx, query, args...)
	d.emit(ctx, "Query", query, start, err)
	return rows, err
}

func (d *otlpDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	res, err := d.inner.ExecContext(ctx, query, args...)
	d.emit(ctx, "Exec", query, start, err)
	return res, err
}

func (d *otlpDB) PingContext(ctx context.Context) error {
	start := time.Now()
	err := d.inner.PingContext(ctx)
	d.emit(ctx, "Ping", "PING", start, err)
	return err
}

func (d *otlpDB) emit(ctx context.Context, kind, query string, start time.Time, err error) {
	// The span name is "sql.Exec" / "sql.Query" — matches what OTel
	// semantic conventions expect, so DOG UI labels stay familiar.
	name := "sql." + kind
	truncated := truncate(query, 80)
	d.sdk.Record(ctx, name, start, err)(
		otlp.String("db.statement", truncated),
	)
	d.sdk.Counter(ctx, "db.calls", 1,
		otlp.String("sql", kind),
		otlp.String("status", statusOf(err)),
	)
	d.sdk.Histogram(ctx, "db.duration_ms",
		float64(time.Since(start).Milliseconds()),
		otlp.String("sql", kind),
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		d.sdk.Log(ctx, otlp.SeverityError,
			"db "+kind+" failed",
			otlp.String("db.statement", truncated),
			otlp.String("error", err.Error()),
		)
	}
}

func statusOf(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	sdk, err := otlp.New(endpoint,
		otlp.WithService("db-tracing-demo"),
		otlp.WithServiceVersion("0.1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	// We do not bring in a real driver here — the demo only needs to
	// exercise the wrapper. Build a stub sql.DB by hitting a closed
	// connection so every call returns a deterministic error and the
	// error path of emit is exercised end-to-end.
	db, err := sql.Open("mysql", "user:pw@tcp(127.0.0.1:1)/db") // will fail to connect
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	wrapped := wrap(sdk, db)

	ctx := context.Background()
	ctx, end := sdk.Trace(ctx, "db.run_queries")
	defer end(nil)

	// Each of these will fail (no real DB); we still exercise the wrapper.
	_, _ = wrapped.QueryContext(ctx, "SELECT id, name FROM users WHERE active = ?", true)
	_, _ = wrapped.ExecContext(ctx, "UPDATE orders SET status = ? WHERE id = ?", "paid", 42)
	errPing := wrapped.PingContext(ctx)
	fmt.Println("ping error:", errPing)

	if err := sdk.ForceFlush(ctx); err != nil {
		log.Printf("flush: %v", err)
	}
}
