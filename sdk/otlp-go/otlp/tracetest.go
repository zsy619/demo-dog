// tracetest -- in-memory collector for SDK tests.
//
// The package provides:
//   - InMemoryCollector: a thread-safe sink that captures every Request
//     the SDK exports, with helpers to filter by service / trace_id /
//     signal type.
//   - NewTestSDK: build an SDK wired to the in-memory collector (with
//     short flush interval so tests do not have to wait).
//
// Usage in your own test:
//
//   func TestSomething(t *testing.T) {
//       sdk, col := tracetest.NewTestSDK(t)
//       defer sdk.Shutdown(context.Background())
//
//       sdk.Log(context.Background(), SeverityInfo, "hi")
//       sdk.ForceFlush(context.Background())
//
//       logs := col.Logs("myservice")
//       if len(logs) != 1 {
//           t.Fatalf("got %d logs", len(logs))
//       }
//   }
package otlp

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// InMemoryCollector captures every Request the SDK emits. Safe for
// concurrent use.
type InMemoryCollector struct {
	mu       sync.Mutex
	requests []Request
	calls    atomic.Int64
}

// NewInMemoryCollector creates a fresh collector.
func NewInMemoryCollector() *InMemoryCollector { return &InMemoryCollector{} }

// Export satisfies ExporterInterface and stores the request in memory.
func (c *InMemoryCollector) Export(_ context.Context, req Request) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	c.calls.Add(1)
	return &Response{
		AcceptedLogs:    len(req.Logs),
		AcceptedMetrics: len(req.Metrics),
		AcceptedSpans:   len(req.Spans),
	}, nil
}

// Requests returns a snapshot of all captured requests.
func (c *InMemoryCollector) Requests() []Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Request, len(c.requests))
	copy(out, c.requests)
	return out
}

// CallCount returns the number of times Export was invoked.
func (c *InMemoryCollector) CallCount() int { return int(c.calls.Load()) }

// Logs returns all log records across all captured requests, optionally
// filtered by service name. Empty service matches everything.
func (c *InMemoryCollector) Logs(service string) []LogRecord {
	var out []LogRecord
	for _, r := range c.Requests() {
		for _, l := range r.Logs {
			if service == "" || l.Service == service {
				out = append(out, l)
			}
		}
	}
	return out
}

// Metrics returns all metric points, optionally filtered by service.
func (c *InMemoryCollector) Metrics(service string) []MetricPoint {
	var out []MetricPoint
	for _, r := range c.Requests() {
		for _, m := range r.Metrics {
			if service == "" || m.Service == service {
				out = append(out, m)
			}
		}
	}
	return out
}

// Spans returns all spans, optionally filtered by trace_id.
func (c *InMemoryCollector) Spans(traceID string) []SpanRecord {
	var out []SpanRecord
	for _, r := range c.Requests() {
		for _, s := range r.Spans {
			if traceID == "" || s.TraceID == traceID {
				out = append(out, s)
			}
		}
	}
	return out
}

// WaitForLogs blocks until at least n logs are captured or timeout fires.
// Returns the captured logs on success, nil on timeout.
func (c *InMemoryCollector) WaitForLogs(n int, timeout time.Duration, service string) []LogRecord {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		logs := c.Logs(service)
		if len(logs) >= n {
			return logs
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

// Reset clears all captured state. Useful between subtests.
func (c *InMemoryCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = nil
	c.calls.Store(0)
}

// NewTestSDK wires an SDK to an InMemoryCollector and registers cleanup.
// The SDK has a 1ms flush interval so tests do not have to wait.
//
//   sdk, col := tracetest.NewTestSDK(t)
//   defer sdk.Shutdown(context.Background())
func NewTestSDK(t *testing.T) (*SDK, *InMemoryCollector) {
	t.Helper()
	col := NewInMemoryCollector()
	sdk, err := New("",
		WithService("test-service"),
		WithFlushInterval(1*time.Millisecond),
		WithExporter(col),
	)
	if err != nil {
		t.Fatalf("tracetest: New: %v", err)
	}
	t.Cleanup(func() {
		_ = sdk.Shutdown(context.Background())
	})
	return sdk, col
}
