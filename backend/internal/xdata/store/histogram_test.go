package store

import (
	"math"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// TestHistogramQuantile verifies that OTel histogram data points with
// explicit bucket boundaries are summed correctly across calls and
// quantile queries return interpolated values from the true bucket
// layout (not log-spaced fallback).
func TestHistogramQuantile(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	bounds := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, math.MaxFloat64}

	// 100 observations of 0.030s, 50 observations of 0.150s, 10 of 0.8s.
	// bucket counts for [0.030]: bucket index 4 (≤0.05) = 100.
	// bucket counts for [0.150]: bucket index 6 (≤0.25) = 50.
	// bucket counts for [0.8]: bucket index 8 (≤1) = 10.
	counts := []int64{0, 0, 0, 0, 100, 0, 50, 0, 10, 0, 0, 0}

	d.InsertMetrics([]model.MetricPoint{{
		Timestamp: now, Service: "svc", Name: "latency", Type: "histogram",
		BucketBounds: bounds, BucketCounts: counts,
		HistogramCount: 160, HistogramSum: 100*0.030 + 50*0.150 + 10*0.8,
	}})

	// Total should be 160 after aggregation.
	got := d.HistogramSnapshot("svc", "latency")
	if got == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if got.Total != 160 {
		t.Fatalf("expected total=160, got %d", got.Total)
	}

	// p50 = observation #80 of 160, which falls in bucket [0.05, 0.1].
	// Linear interpolation: 0.1 - 0.2 * (0.1 - 0.05) = 0.09.
	p50 := d.HistogramQuantile("svc", "latency", 0.5)
	if p50 < 0.05 || p50 > 0.1 {
		t.Fatalf("expected p50 in [0.05,0.1], got %v", p50)
	}

	// p95 = observation #152 of 160, which falls in bucket [1, 2.5].
	// 160 - 152 = 8 of 10 bucket counts past target; interpolation:
	// 2.5 - 0.8 * (2.5 - 1) = 1.3.
	p95 := d.HistogramQuantile("svc", "latency", 0.95)
	if p95 < 1 || p95 > 2.5 {
		t.Fatalf("expected p95 in [1, 2.5], got %v", p95)
	}

	// p99 = observation #158 of 160, also bucket [1, 2.5].
	// 160 - 158 = 2 of 10; 2.5 - 0.2 * (2.5 - 1) = 2.1.
	p99 := d.HistogramQuantile("svc", "latency", 0.99)
	if p99 < 1 || p99 > 2.5 {
		t.Fatalf("expected p99 in [1, 2.5], got %v", p99)
	}
}

// TestHistogramQuantileEmpty verifies a brand-new store returns 0.
func TestHistogramQuantileEmpty(t *testing.T) {
	d := New(DefaultConfig())
	if q := d.HistogramQuantile("ghost", "x", 0.99); q != 0 {
		t.Fatalf("expected 0 for missing series, got %v", q)
	}
}

// TestHistogramAggMerges verifies that two data points with the same
// bucket bounds accumulate, not replace.
func TestHistogramAggMerges(t *testing.T) {
	d := New(DefaultConfig())
	now := time.Now()
	bounds := []float64{1, 2, 5, 10, math.MaxFloat64}
	countsA := []int64{1, 2, 3, 4, 5}
	countsB := []int64{5, 4, 3, 2, 1}

	d.InsertMetrics([]model.MetricPoint{{
		Timestamp: now, Service: "s", Name: "n", Type: "histogram",
		BucketBounds: bounds, BucketCounts: countsA, HistogramCount: 15,
	}})
	d.InsertMetrics([]model.MetricPoint{{
		Timestamp: now, Service: "s", Name: "n", Type: "histogram",
		BucketBounds: bounds, BucketCounts: countsB, HistogramCount: 15,
	}})

	got := d.HistogramSnapshot("s", "n")
	if got == nil {
		t.Fatal("expected snapshot")
	}
	if got.Total != 30 {
		t.Fatalf("expected total=30 after merge, got %d", got.Total)
	}
	expectedCounts := []int64{6, 6, 6, 6, 6}
	for i, c := range expectedCounts {
		if got.Counts[i] != c {
			t.Fatalf("bucket %d: expected %d, got %d", i, c, got.Counts[i])
		}
	}
}
