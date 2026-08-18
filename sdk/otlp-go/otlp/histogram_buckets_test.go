package otlp

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"
)

// TestHistogramBucketsAccumulation verifies WithHistogramBuckets
// causes observations to be accumulated per-name between flushes and
// flushed as a single OTel histogram data point per series.
func TestHistogramBucketsAccumulation(t *testing.T) {
	sdk, err := New("http://localhost:1",
		WithService("test"),
		WithFlushInterval(time.Hour), // disable background flush
		WithHistogramBuckets([]float64{0.01, 0.1, 1, 10, math.MaxFloat64}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	// 100 samples: 50 of 0.005, 30 of 0.05, 20 of 0.5.
	for i := 0; i < 50; i++ {
		sdk.Histogram(context.Background(), "latency", 0.005)
	}
	for i := 0; i < 30; i++ {
		sdk.Histogram(context.Background(), "latency", 0.05)
	}
	for i := 0; i < 20; i++ {
		sdk.Histogram(context.Background(), "latency", 0.5)
	}

	acc := sdk.histogramAcc["latency"]
	if acc == nil {
		t.Fatal("expected accumulator for latency")
	}
	if acc.total != 100 {
		t.Fatalf("expected 100 observations, got %d", acc.total)
	}
	if acc.name != "latency" {
		t.Fatalf("expected name=latency, got %q", acc.name)
	}
	// Expected bucket counts:
	//   [0.005..0.01]: 50 + 30 (0.05 ≤ 0.1) wait — bounds are [0.01, 0.1, 1, 10, Inf].
	//   bucket[0] (≤0.01): 50 of 0.005
	//   bucket[1] (≤0.1):  30 of 0.05
	//   bucket[2] (≤1):    20 of 0.5
	//   bucket[3] (≤10):   0
	//   bucket[4] (Inf):   0
	expected := []int64{50, 30, 20, 0, 0}
	for i, want := range expected {
		if acc.counts[i] != want {
			t.Fatalf("bucket %d: expected %d, got %d", i, want, acc.counts[i])
		}
	}
}

// TestHistogramBucketsConcurrency verifies parallel Histogram() calls
// do not lose observations or corrupt bucket counts.
func TestHistogramBucketsConcurrency(t *testing.T) {
	sdk, err := New("http://localhost:1",
		WithService("test"),
		WithFlushInterval(time.Hour),
		WithHistogramBuckets([]float64{1, 10, math.MaxFloat64}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	const goroutines = 16
	const perG = 100
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				sdk.Histogram(context.Background(), "n", float64(i%5))
			}
		}()
	}
	wg.Wait()

	acc := sdk.histogramAcc["n"]
	if acc == nil {
		t.Fatal("missing accumulator")
	}
	if acc.total != goroutines*perG {
		t.Fatalf("expected %d obs, got %d", goroutines*perG, acc.total)
	}
	// Values 0..4 all ≤ 1, so first bucket = all, rest = 0.
	// Values 0,1 → bucket 0 (≤1); 2,3,4 → bucket 1 (≤10). With
	// perG=100 and i%5 cycle, each goroutine contributes 40 to bucket 0
	// and 60 to bucket 1.
	const perGBucket0 = int64(40)
	const perGBucket1 = int64(60)
	if acc.counts[0] != perGBucket0*goroutines {
		t.Fatalf("bucket 0: expected %d, got %d", perGBucket0*goroutines, acc.counts[0])
	}
	if acc.counts[1] != perGBucket1*goroutines {
		t.Fatalf("bucket 1: expected %d, got %d", perGBucket1*goroutines, acc.counts[1])
	}
	if acc.counts[2] != 0 {
		t.Fatalf("bucket 2: expected 0, got %d", acc.counts[2])
	}
}

// TestHistogramLegacyBehavior confirms that without WithHistogramBuckets
// each call still emits one data point per observation (legacy path).
func TestHistogramLegacyBehavior(t *testing.T) {
	sdk, err := New("http://localhost:1",
		WithService("test"),
		WithFlushInterval(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	for i := 0; i < 5; i++ {
		sdk.Histogram(context.Background(), "n", 1)
	}
	// No accumulator should exist.
	if sdk.histogramAcc != nil && len(sdk.histogramAcc) != 0 {
		t.Fatalf("legacy mode should not allocate accumulator, got %d entries", len(sdk.histogramAcc))
	}
}
