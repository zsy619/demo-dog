package api

import (
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
)

func TestEvalPromQL_Selector(t *testing.T) {
	d := store.New(store.DefaultConfig())
	now := time.Now()
	d.InsertMetrics([]model.MetricPoint{
		{Timestamp: now.Add(-1 * time.Minute), TenantID: "", Service: "checkout", Name: "orders.placed", Value: 3},
		{Timestamp: now, TenantID: "", Service: "checkout", Name: "orders.placed", Value: 5},
	})
	r, err := evalPromQL("orders.placed", now, "", d)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Data.Result) != 1 {
		t.Fatalf("want 1 series, got %d", len(r.Data.Result))
	}
	if got := r.Data.Result[0].Metric["name"]; got != "orders.placed" {
		t.Fatalf("got %q", got)
	}
}

func TestEvalPromQL_SumAggregator(t *testing.T) {
	d := store.New(store.DefaultConfig())
	now := time.Now()
	d.InsertMetrics([]model.MetricPoint{
		{Timestamp: now.Add(-2 * time.Minute), TenantID: "", Service: "checkout", Name: "sumtest1", Value: 1},
		{Timestamp: now.Add(-1 * time.Minute), TenantID: "", Service: "checkout", Name: "sumtest1", Value: 2},
		{Timestamp: now.Add(-3 * time.Minute), TenantID: "", Service: "inventory", Name: "sumtest1", Value: 4},
	})
	_, snap, _ := d.Snapshot()
	t.Logf("snap len=%d", len(snap))
	for _, m := range snap { t.Logf("  %+v", m) }
	r, err := evalPromQL("sum by (service) (sumtest1)", now, "", d)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("eval result: %+v", r)
	if len(r.Data.Result) != 2 {
		t.Fatalf("want 2 series, got %d", len(r.Data.Result))
	}
	for _, s := range r.Data.Result {
		switch s.Metric["service"] {
		case "checkout":
			if v := s.Value[1].(string); v != "3.0000" {
				t.Fatalf("checkout sum: %s", v)
			}
		case "inventory":
			if v := s.Value[1].(string); v != "4.0000" {
				t.Fatalf("inventory sum: %s", v)
			}
		}
	}
}

func TestEvalPromQL_HistogramQuantile(t *testing.T) {
	d := store.New(store.DefaultConfig())
	now := time.Now()
	var samples []model.MetricPoint
	for i, v := range []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		samples = append(samples, model.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			TenantID:  "",
			Service:   "checkout",
			Name:      "latency",
			Value:     v,
		})
	}
	d.InsertMetrics(samples)
	r, err := evalPromQL("histogram_quantile(0.5, latency)", now, "", d)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Data.Result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r.Data.Result))
	}
}

func TestParseSelector(t *testing.T) {
	name, labels := parseSelector(`cpu{service="checkout"}`)
	if name != "cpu" {
		t.Fatalf("name: %q", name)
	}
	if labels["service"] != "checkout" {
		t.Fatalf("labels: %v", labels)
	}
}

func TestExtractWindow(t *testing.T) {
	dur, inner, err := extractWindow("rate(cpu[5m])")
	if err != nil {
		t.Fatal(err)
	}
	if dur != 5*time.Minute {
		t.Fatalf("dur: %v", dur)
	}
	if inner != "cpu" {
		t.Fatalf("inner: %q", inner)
	}
}

func TestEvalPromQL_TenantFilter(t *testing.T) {
	d := store.New(store.DefaultConfig())
	now := time.Now()
	d.InsertMetrics([]model.MetricPoint{
		{Timestamp: now, TenantID: "acme", Service: "checkout", Name: "x", Value: 1},
		{Timestamp: now, TenantID: "globex", Service: "checkout", Name: "x", Value: 2},
	})
	r, err := evalPromQL("x", now, "acme", d)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Data.Result) != 1 {
		t.Fatalf("expected only acme series, got %d", len(r.Data.Result))
	}
}
