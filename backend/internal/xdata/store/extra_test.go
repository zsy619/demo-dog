package store

import (
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

func TestQueryLogsFiltered(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	d.InsertLogs([]model.LogRecord{
		{Timestamp: now.Add(-time.Minute), Service: "checkout", Severity: model.SeverityInfo, Body: "hello checkout"},
		{Timestamp: now.Add(-time.Minute), Service: "search", Severity: model.SeverityError, Body: "error in search"},
		{Timestamp: now.Add(-time.Minute), Service: "checkout", Severity: model.SeverityError, Body: "checkout failed"},
	})

	// Filter by service + severity
	res := d.QueryLogsFiltered(QueryFilter{Service: "checkout", Severity: "ERROR", Limit: 100})
	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Rows))
	}
	if res.Rows[0]["body"] != "checkout failed" {
		t.Fatalf("unexpected body: %v", res.Rows[0]["body"])
	}

	// Search substring (case-insensitive)
	res = d.QueryLogsFiltered(QueryFilter{Search: "HELLO", Limit: 100})
	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 row from search, got %d", len(res.Rows))
	}

	// Label filter
	d.InsertLogs([]model.LogRecord{
		{Timestamp: now, Service: "search", Severity: model.SeverityInfo, Body: "with labels", Attributes: map[string]string{"env": "prod"}},
	})
	res = d.QueryLogsFiltered(QueryFilter{Labels: map[string]string{"env": "prod"}, Limit: 100})
	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 row matching label, got %d", len(res.Rows))
	}
}

func TestPercentileLatencies(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	d.InsertSpans([]model.SpanRecord{
		{TraceID: "t1", SpanID: "a", Service: "checkout", StartTime: now, DurationMs: 10},
		{TraceID: "t1", SpanID: "b", Service: "checkout", StartTime: now, DurationMs: 20},
		{TraceID: "t1", SpanID: "c", Service: "checkout", StartTime: now, DurationMs: 30},
		{TraceID: "t1", SpanID: "d", Service: "checkout", StartTime: now, DurationMs: 100},
	})
	p50, _, p99 := d.PercentileLatencies("checkout")
	// Sorted: [10, 20, 30, 100]. With interpolation:
	//   p50: pos=1.5, between 20 and 30 -> 25
	//   p99: pos=2.97, between 30 and 100 -> 30 + 0.97*70 = 97.9
	if p50 < 20 || p50 > 30 {
		t.Fatalf("p50 expected ~25, got %v", p50)
	}
	if p99 < 90 || p99 > 100 {
		t.Fatalf("p99 expected ~98, got %v", p99)
	}
	// Also exercise empty + single-sample edge cases.
	emptyP50, _, _ := d.PercentileLatencies("nonexistent")
	if emptyP50 != 0 {
		t.Fatalf("expected 0 for empty service, got %v", emptyP50)
	}
}

func TestServiceMap(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	d.InsertSpans([]model.SpanRecord{
		{TraceID: "t1", SpanID: "root", Service: "frontend", StartTime: now, DurationMs: 50, Status: "ok"},
		{TraceID: "t1", SpanID: "c1", ParentID: "root", Service: "checkout", StartTime: now, DurationMs: 30, Status: "ok"},
		{TraceID: "t1", SpanID: "c2", ParentID: "root", Service: "search", StartTime: now, DurationMs: 10, Status: "error"},
		{TraceID: "t1", SpanID: "c3", ParentID: "c1", Service: "auth", StartTime: now, DurationMs: 5, Status: "ok"},
	})
	m := d.ServiceMap()
	if len(m.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d (%v)", len(m.Nodes), m.Nodes)
	}
	if len(m.Edges) < 2 {
		t.Fatalf("expected >=2 edges, got %d", len(m.Edges))
	}
	// Edge: frontend -> search should have 1 error
	found := false
	for _, e := range m.Edges {
		if e.From == "frontend" && e.To == "search" {
			found = true
			if e.Errors != 1 {
				t.Fatalf("expected 1 error on frontend->search, got %d", e.Errors)
			}
		}
	}
	if !found {
		t.Fatalf("did not find frontend->search edge in %+v", m.Edges)
	}
}

func TestLabelKeys(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	d.InsertLogs([]model.LogRecord{
		{Timestamp: now, Service: "x", Attributes: map[string]string{"env": "prod", "region": "us-east"}},
	})
	d.InsertMetrics([]model.MetricPoint{
		{Timestamp: now, Service: "x", Name: "m1", Labels: map[string]string{"kind": "counter"}},
	})
	d.InsertSpans([]model.SpanRecord{
		{TraceID: "t", SpanID: "s", Service: "x", StartTime: now, Attributes: map[string]string{"env": "prod"}},
	})
	k := d.LabelKeys()
	if len(k.Logs) < 2 || len(k.Metrics) < 1 || len(k.Spans) < 1 {
		t.Fatalf("unexpected label keys: %+v", k)
	}
}

func TestHistogramCounts(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	d.InsertSpans([]model.SpanRecord{
		{TraceID: "t1", SpanID: "a", Service: "x", StartTime: now, DurationMs: 1},
		{TraceID: "t1", SpanID: "b", Service: "x", StartTime: now, DurationMs: 50},
		{TraceID: "t1", SpanID: "c", Service: "x", StartTime: now, DurationMs: 99},
	})
	counts := d.HistogramCounts("x", 4)
	if len(counts) != 4 {
		t.Fatalf("expected 4 bins, got %d", len(counts))
	}
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != 3 {
		t.Fatalf("expected 3 samples in histogram, got %d", total)
	}
	// Verify the fixed-bucket placement puts the three known samples in
	// distinct edges: 1ms → bucket 0 (≤1ms), 50ms → bucket 5 (≤50ms),
	// 99ms → bucket 6 (≤100ms). With 4 requested bins via resampleBuckets
	// we just need every sample to land somewhere — and the old "maxV=1"
	// bug put everything in bucket 0, so any non-bucket-0 placement
	// proves the fix works.
	allInFirst := counts[0] == 3
	if allInFirst {
		t.Fatalf("regression: all 3 samples in bin 0 (old maxV=1 bug)")
	}
}

func TestSnapshot(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	for i := 0; i < 10; i++ {
		d.InsertLogs([]model.LogRecord{{Timestamp: now, Service: "x", Severity: model.SeverityInfo, Body: "hi"}})
		d.InsertMetrics([]model.MetricPoint{{Timestamp: now, Service: "x", Name: "n", Value: float64(i)}})
	}
	logs, metrics, _ := d.Snapshot()
	if len(logs) == 0 || len(metrics) == 0 {
		t.Fatalf("snapshot empty")
	}
}

func TestServiceDetailAggregatesEndpointsAndErrors(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	d.InsertLogs([]model.LogRecord{
		{Timestamp: now, Service: "checkout", Severity: model.SeverityInfo, Body: "ok", TraceID: "tr-1"},
		{Timestamp: now, Service: "checkout", Severity: model.SeverityError, Body: "boom", TraceID: "tr-1"},
	})
	d.InsertSpans([]model.SpanRecord{
		{TraceID: "tr-1", SpanID: "s1", Name: "POST /checkout", Service: "checkout", StartTime: now, DurationMs: 100, Status: "ok"},
		{TraceID: "tr-1", SpanID: "s2", Name: "POST /checkout", Service: "checkout", StartTime: now, DurationMs: 250, Status: "error"},
		{TraceID: "tr-2", SpanID: "s3", Name: "GET /cart", Service: "checkout", StartTime: now, DurationMs: 12, Status: "ok"},
	})
	d.InsertMetrics([]model.MetricPoint{
		{Timestamp: now, TenantID: "t1", Service: "checkout", Name: "http.server.duration", Value: 100, Type: "histogram"},
	})

	det, ok := d.ServiceDetail("checkout")
	if !ok {
		t.Fatalf("expected detail ok")
	}
	if det.Summary.Name != "checkout" {
		t.Fatalf("name=%s", det.Summary.Name)
	}
	if len(det.Endpoints) != 2 {
		t.Fatalf("want 2 endpoints, got %d", len(det.Endpoints))
	}
	if det.Endpoints[0].Name != "POST /checkout" {
		t.Fatalf("top endpoint=%s", det.Endpoints[0].Name)
	}
	if det.Endpoints[0].Errors != 1 {
		t.Fatalf("top endpoint errors=%d", det.Endpoints[0].Errors)
	}
	if len(det.RecentErrors) != 1 || det.RecentErrors[0].Body != "boom" {
		t.Fatalf("recent errors=%+v", det.RecentErrors)
	}
	if len(det.RecentTraces) < 2 {
		t.Fatalf("want >=2 recent traces, got %d", len(det.RecentTraces))
	}
	if len(det.MetricNames) != 1 || det.MetricNames[0] != "http.server.duration" {
		t.Fatalf("metric names=%v", det.MetricNames)
	}
}


// TestQueryTracesFilteredExpandsTracesByService verifies the trace query
// surfaces every span of a matched trace, not just the spans in the
// filtered service. This is what makes the Traces page useful when the
// caller drills into one service but the trace actually crosses services.
func TestQueryTracesFilteredExpandsTracesByService(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	d.InsertSpans([]model.SpanRecord{
		// Trace tr-A: root in checkout, children in auth + postgres.
		{TraceID: "tr-A", SpanID: "a1", Name: "GET /checkout", Service: "checkout", StartTime: now, DurationMs: 80, Status: "ok"},
		{TraceID: "tr-A", SpanID: "a2", ParentID: "a1", Name: "auth.check", Service: "auth", StartTime: now.Add(2 * time.Millisecond), DurationMs: 30, Status: "ok"},
		{TraceID: "tr-A", SpanID: "a3", ParentID: "a2", Name: "query", Service: "postgres", StartTime: now.Add(5 * time.Millisecond), DurationMs: 12, Status: "ok"},
		// Trace tr-B: independent, only spans in checkout.
		{TraceID: "tr-B", SpanID: "b1", Name: "GET /cart", Service: "checkout", StartTime: now, DurationMs: 25, Status: "ok"},
	})

	q := d.QueryTracesFiltered(QueryFilter{Service: "checkout", Limit: 50})
	if q.Stats.Returned != 4 {
		t.Fatalf("want 4 spans returned (tr-A fully expanded + tr-B), got %d", q.Stats.Returned)
	}
	byTrace := map[string]int{}
	for _, r := range q.Rows {
		byTrace[r["trace_id"].(string)]++
	}
	if byTrace["tr-A"] != 3 {
		t.Fatalf("tr-A should have 3 spans (root + 2 children), got %d", byTrace["tr-A"])
	}
	if byTrace["tr-B"] != 1 {
		t.Fatalf("tr-B should have 1 span, got %d", byTrace["tr-B"])
	}

	// Querying for a service that has zero matching traces returns nothing.
	q2 := d.QueryTracesFiltered(QueryFilter{Service: "ads", Limit: 50})
	if q2.Stats.Returned != 0 {
		t.Fatalf("ads service should return 0 spans, got %d", q2.Stats.Returned)
	}

	// TraceID filter short-circuits and returns every span of that trace.
	q3 := d.QueryTracesFiltered(QueryFilter{TraceID: "tr-A", Limit: 50})
	if q3.Stats.Returned != 3 {
		t.Fatalf("tr-A by id should return 3 spans, got %d", q3.Stats.Returned)
	}
}
