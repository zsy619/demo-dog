package store

import (
	"bytes"
	"testing"
	"time"


	"github.com/zsy619/demo-dog/backend/internal/model"
)

func TestSnapshot_RoundTripHistogram(t *testing.T) {
	d1 := New(DefaultConfig())
	now := time.Now()
	d1.InsertMetrics([]model.MetricPoint{{
		Service:   "checkout",
		Name:      "http.server.duration",
		Timestamp: now,
		Value:     1,
		Type:      "histogram",
		BucketBounds: []float64{1, 5, 10, 50, 100},
		BucketCounts: []int64{2, 3, 1, 1, 1},
		HistogramCount: 8,
		HistogramSum:   50,
		HistogramMin:   1,
		HistogramMax:   100,
	}})
	d1.InsertMetrics([]model.MetricPoint{
		{Service: "checkout", Name: "http.server.duration", Timestamp: now, Value: 12},
		{Service: "checkout", Name: "http.server.duration", Timestamp: now, Value: 25},
	})

	data, err := d1.PersistSnapshotBytes()
	if err != nil {
		t.Fatal(err)
	}

	d2 := New(DefaultConfig())
	if err := d2.RestoreSnapshot(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}

	// Histogram aggregate should be back.
	h := d2.HistogramSnapshot("checkout", "http.server.duration")
	if h == nil {
		t.Fatal("histogram missing after restore")
	}
	if len(h.Bounds) != 5 || h.Counts[0] != 2 {
		t.Fatalf("bounds/counts: %+v", h)
	}

	// t-digest path should also work: quantile from raw observations.
	q := d2.HistogramQuantile("checkout", "http.server.duration", 0.5)
	if q <= 0 {
		t.Fatalf("quantile should be > 0, got %v", q)
	}
}

func TestTDigest_Restore_MatchesOriginal(t *testing.T) {
	d := NewTDigest(200)
	for i := 0; i < 10000; i++ {
		d.Observe(float64(i))
	}
	cents, total, min, max := d.Snapshot()
	if total != 10000 {
		t.Fatalf("snapshot total: %d", total)
	}
	r := NewTDigest(200)
	r.Restore(cents, total, min, max)
	for _, q := range []float64{0.5, 0.9, 0.99} {
		if got := d.Quantile(q); got != r.Quantile(q) {
			t.Errorf("q=%v drift: original=%v restored=%v", q, got, r.Quantile(q))
		}
	}
}
