package buffer

import (
	"testing"
	"time"
)

func TestPushDrainRoundtrip(t *testing.T) {
	b := New("checkout", map[string]string{"service.name": "checkout"})
	now := time.Now()
	b.PushLog(LogRecord{Timestamp: now, Body: "hello", Severity: "INFO"})
	b.PushMetric(MetricPoint{Timestamp: now, Name: "x", Value: 1, Type: "counter"})
	b.PushSpan(SpanRecord{TraceID: "t1", SpanID: "s1", StartTime: now, DurationMs: 5, Status: "ok"})

	logs, metrics, spans := b.Size()
	if logs != 1 || metrics != 1 || spans != 1 {
		t.Fatalf("size mismatch: %d/%d/%d", logs, metrics, spans)
	}

	req := b.Drain()
	if len(req.Logs) != 1 || len(req.Metrics) != 1 || len(req.Spans) != 1 {
		t.Fatalf("drain mismatch: %+v", req)
	}
	if req.ResourceAttrs["service.name"] != "checkout" {
		t.Fatalf("resource attr missing: %+v", req.ResourceAttrs)
	}

	logs, metrics, spans = b.Size()
	if logs != 0 || metrics != 0 || spans != 0 {
		t.Fatalf("expected empty after drain: %d/%d/%d", logs, metrics, spans)
	}
}

func TestDefaultsInjected(t *testing.T) {
	b := New("orders", nil)
	b.PushLog(LogRecord{Body: "x"})
	req := b.Drain()
	if req.Logs[0].Service != "orders" {
		t.Fatalf("service default missing: %q", req.Logs[0].Service)
	}
	if req.Logs[0].Timestamp.IsZero() {
		t.Fatalf("timestamp default missing")
	}
}

func TestConcurrent(t *testing.T) {
	b := New("svc", nil)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			b.PushLog(LogRecord{Body: "x"})
		}
		close(done)
	}()
	for i := 0; i < 1000; i++ {
		b.PushMetric(MetricPoint{Name: "y", Value: float64(i), Type: "gauge"})
	}
	<-done
	req := b.Drain()
	if len(req.Logs) != 1000 || len(req.Metrics) != 1000 {
		t.Fatalf("concurrent push mismatch: %d/%d", len(req.Logs), len(req.Metrics))
	}
}
