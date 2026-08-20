package store

import (
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

func TestInsertAndQueryLogs(t *testing.T) {
	d := New(DefaultConfig())
	req := []model.LogRecord{
		{Timestamp: time.Now(), Service: "checkout", Severity: model.SeverityInfo, Body: "ok"},
		{Timestamp: time.Now(), Service: "checkout", Severity: model.SeverityError, Body: "boom"},
		{Timestamp: time.Now(), Service: "search", Severity: model.SeverityInfo, Body: "ok"},
	}
	if got := d.InsertLogs(req); got != 3 {
		t.Fatalf("expected 3 inserted, got %d", got)
	}
	out := d.QueryLogs("checkout", "", 10, 0)
	if len(out.Rows) != 2 {
		t.Fatalf("expected 2 rows for checkout, got %d", len(out.Rows))
	}
	if out.Rows[0]["service"] != "checkout" {
		t.Fatalf("unexpected service: %v", out.Rows[0]["service"])
	}
}

func TestInsertAndQueryMetrics(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	// Insert metrics across 5 different minutes so the 1m MV produces 5 buckets.
	for i := 0; i < 5; i++ {
		d.InsertMetrics([]model.MetricPoint{
			{Timestamp: now.Add(time.Duration(i) * time.Minute), Service: "checkout", Name: "http.server.duration", TenantID: "t1", Value: float64(i * 10)},
		})
	}
	out := d.QueryMetrics("t1", "checkout", "http.server.duration", "1m", 10)
	if len(out.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(out.Series))
	}
	if len(out.Series[0].Points) != 5 {
		t.Fatalf("expected 5 points, got %d", len(out.Series[0].Points))
	}
}

func TestInsertAndQuerySpans(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	d.InsertSpans([]model.SpanRecord{
		{TraceID: "abc", SpanID: "s1", Service: "checkout", StartTime: now, DurationMs: 50, Status: "ok"},
		{TraceID: "abc", SpanID: "s2", ParentID: "s1", Service: "checkout", StartTime: now, DurationMs: 20, Status: "ok"},
	})
	out := d.QueryTraces("abc", "", 10)
	if len(out.Rows) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(out.Rows))
	}
}

func TestServiceSummary(t *testing.T) {
	d := New(DefaultConfig())
	d.InsertLogs([]model.LogRecord{
		{Timestamp: time.Now(), Service: "checkout", Severity: model.SeverityInfo, Body: "ok"},
		{Timestamp: time.Now(), Service: "checkout", Severity: model.SeverityError, Body: "boom"},
		{Timestamp: time.Now(), Service: "checkout", Severity: model.SeverityError, Body: "boom"},
	})
	svcs := d.ListServices("")
	if len(svcs) != 1 {
		t.Fatalf("expected 1 service, got %d", len(svcs))
	}
	if svcs[0].Name != "checkout" {
		t.Fatalf("unexpected service: %s", svcs[0].Name)
	}
	if svcs[0].LogsCount != 3 {
		t.Fatalf("expected 3 logs, got %d", svcs[0].LogsCount)
	}
	if svcs[0].ErrorRate <= 0 {
		t.Fatalf("expected positive error rate, got %f", svcs[0].ErrorRate)
	}
}
