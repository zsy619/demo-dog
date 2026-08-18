package buffer

import (
	"fmt"
	"testing"
	"time"
)

func TestBuffer_CapDropsOldest(t *testing.T) {
	b := New("svc", nil, WithCaps(3, 3, 3))

	// Push 5 logs into a 3-cap buffer; the first 2 must be dropped.
	for i := 0; i < 5; i++ {
		b.PushLog(LogRecord{Timestamp: time.Unix(int64(i), 0), Body: fmt.Sprintf("l%d", i)})
	}

	logs, _, _ := b.Size()
	if logs != 3 {
		t.Fatalf("expected 3 buffered logs, got %d", logs)
	}
	dl, dm, ds := b.Stats()
	if dl != 2 {
		t.Errorf("expected 2 dropped logs, got %d", dl)
	}
	if dm != 0 || ds != 0 {
		t.Errorf("expected 0 drops for other streams")
	}

	// The oldest two must have been dropped; the kept three start at l2.
	req := b.Drain()
	if req.Logs[0].Body != "l2" || req.Logs[2].Body != "l4" {
		t.Errorf("expected [l2,l3,l4], got %s/%s/%s",
			req.Logs[0].Body, req.Logs[1].Body, req.Logs[2].Body)
	}
}

func TestBuffer_ZeroCapIsUnbounded(t *testing.T) {
	b := New("svc", nil)
	for i := 0; i < 100; i++ {
		b.PushLog(LogRecord{Body: "x"})
	}
	logs, _, _ := b.Size()
	if logs != 100 {
		t.Fatalf("expected 100, got %d", logs)
	}
	dl, _, _ := b.Stats()
	if dl != 0 {
		t.Errorf("unbounded buffer should not drop")
	}
}

func TestBuffer_CapPerStream(t *testing.T) {
	b := New("svc", nil, WithCaps(100, 2, 0))
	for i := 0; i < 5; i++ {
		b.PushMetric(MetricPoint{Name: "m", Value: float64(i)})
	}
	_, metrics, _ := b.Size()
	if metrics != 2 {
		t.Errorf("expected metric cap 2, got %d", metrics)
	}
	for i := 0; i < 50; i++ {
		b.PushSpan(SpanRecord{SpanID: "s", DurationMs: 1})
	}
	_, _, spans := b.Size()
	if spans != 50 {
		t.Errorf("expected 50 spans, got %d", spans)
	}
}
