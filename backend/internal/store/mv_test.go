package store

import (
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/model"
)

// TestMetricMVAveragesCorrectly verifies the MV (materialized view)
// rollup keeps a proper mean via sum/count rather than the old
// running-average hack that biased every bucket toward the first
// sample ever seen.
func TestMetricMVAveragesCorrectly(t *testing.T) {
	d := New(DefaultConfig())
	// All samples fall in the same 1-minute bucket.
	ts := time.Now().Truncate(time.Minute)
	d.InsertMetrics([]model.MetricPoint{
		{Timestamp: ts, Service: "svc", Name: "cpu", Value: 10},
		{Timestamp: ts.Add(10 * time.Second), Service: "svc", Name: "cpu", Value: 20},
		{Timestamp: ts.Add(20 * time.Second), Service: "svc", Name: "cpu", Value: 30},
		{Timestamp: ts.Add(30 * time.Second), Service: "svc", Name: "cpu", Value: 40},
	})
	// Mean of [10,20,30,40] = 25. The old code would have returned
	// ((10+20)/2 + 30)/2 + 40)/2 = 16.25 — biased toward the first sample.
	got := d.mvMinute["svc|cpu"]
	if len(got) != 1 {
		t.Fatalf("expected 1 MV bucket, got %d", len(got))
	}
	if got[0].Count != 4 {
		t.Fatalf("expected count=4, got %d", got[0].Count)
	}
	mean := got[0].Mean()
	if mean < 24.99 || mean > 25.01 {
		t.Fatalf("expected mean=25, got %v", mean)
	}
	// Min/max should track endpoints.
	if got[0].Min != 10 {
		t.Fatalf("expected min=10, got %v", got[0].Min)
	}
	if got[0].Max != 40 {
		t.Fatalf("expected max=40, got %v", got[0].Max)
	}
}

// TestMetricMVMultipleBuckets verifies each minute gets its own
// bucket and they are read back in order.
func TestMetricMVMultipleBuckets(t *testing.T) {
	d := New(DefaultConfig())
	base := time.Now().Truncate(time.Minute)
	d.InsertMetrics([]model.MetricPoint{
		{Timestamp: base, Service: "svc", Name: "cpu", Value: 1},
		{Timestamp: base.Add(time.Minute), Service: "svc", Name: "cpu", Value: 2},
		{Timestamp: base.Add(2 * time.Minute), Service: "svc", Name: "cpu", Value: 3},
	})
	buckets := d.mvMinute["svc|cpu"]
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}
	for i, want := range []float64{1, 2, 3} {
		if buckets[i].Mean() != want {
			t.Fatalf("bucket %d: expected %v, got %v", i, want, buckets[i].Mean())
		}
	}
}
