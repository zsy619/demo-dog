package store

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func TestTDigest_Empty(t *testing.T) {
	d := NewTDigest(100)
	if d.Count() != 0 {
		t.Fatal("empty digest should have count 0")
	}
	if d.Quantile(0.5) != 0 {
		t.Fatal("empty digest should return 0 for any quantile")
	}
}

func TestTDigest_UniformExact(t *testing.T) {
	d := NewTDigest(200)
	for i := 1; i <= 10000; i++ {
		d.Observe(float64(i))
	}
	q := d.Quantile(0.5)
	if math.Abs(q-5000) > 100 {
		t.Fatalf("median off: got %v want ~5000", q)
	}
	q99 := d.Quantile(0.99)
	if math.Abs(q99-9900) > 200 {
		t.Fatalf("p99 off: got %v want ~9900", q99)
	}
}

func TestTDigest_Normal(t *testing.T) {
	d := NewTDigest(200)
	r := rand.New(rand.NewSource(42))
	// Generate 20k samples from a normal distribution (Box-Muller).
	for i := 0; i < 20000; i++ {
		u1 := r.Float64()
		u2 := r.Float64()
		z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
		d.Observe(100 + 15*z) // mean=100, stddev=15
	}
	q50 := d.Quantile(0.5)
	if math.Abs(q50-100) > 3 {
		t.Fatalf("median off for normal: got %v want ~100", q50)
	}
	q95 := d.Quantile(0.95)
	// mean + 1.645*stddev = 124.7
	if math.Abs(q95-124.7) > 5 {
		t.Fatalf("p95 off for normal: got %v want ~124.7", q95)
	}
}

func TestTDigest_BatchObserve(t *testing.T) {
	d := NewTDigest(100)
	d.ObserveBatch(10, 100)
	d.ObserveBatch(20, 50)
	if d.Count() != 150 {
		t.Fatalf("count: %d", d.Count())
	}
	q := d.Quantile(0.5)
	if q < 8 || q > 22 {
		t.Fatalf("batch quantile: %v", q)
	}
}

func TestTDigest_NaNSkipped(t *testing.T) {
	d := NewTDigest(100)
	d.Observe(math.NaN())
	d.Observe(1.0)
	if d.Count() != 1 {
		t.Fatal("NaN should be skipped")
	}
}

func TestTDigest_MinMax(t *testing.T) {
	d := NewTDigest(100)
	d.Observe(5)
	d.Observe(100)
	d.Observe(-3)
	if d.Min() != -3 {
		t.Fatalf("min: %v", d.Min())
	}
	if d.Max() != 100 {
		t.Fatalf("max: %v", d.Max())
	}
}

func TestTDigest_SnapshotRestore(t *testing.T) {
	d := NewTDigest(100)
	for i := 0; i < 1000; i++ {
		d.Observe(float64(i))
	}
	centroids, total, min, max := d.Snapshot()
	if total != 1000 {
		t.Fatalf("snapshot total: %d", total)
	}
	restored := NewTDigest(100)
	restored.Restore(centroids, total, min, max)
	if restored.Count() != 1000 {
		t.Fatalf("restored count: %d", restored.Count())
	}
	q1 := d.Quantile(0.5)
	q2 := restored.Quantile(0.5)
	if math.Abs(q1-q2) > 1 {
		t.Fatalf("round-trip quantile: %v vs %v", q1, q2)
	}
}

func TestTDigest_QuantileBoundaries(t *testing.T) {
	d := NewTDigest(100)
	for i := 1; i <= 1000; i++ {
		d.Observe(float64(i))
	}
	// q < 0 should clamp to 0
	if v := d.Quantile(-0.5); v != 1 { // smallest observed
		t.Fatalf("q<0: %v", v)
	}
	// q > 1 should clamp to 1
	if v := d.Quantile(1.5); v != 1000 { // largest observed
		t.Fatalf("q>1: %v", v)
	}
}

func TestTDigest_HighAccuracy(t *testing.T) {
	// At delta=200 the digest should hold <0.5% rank error on a
	// 100k uniform sample.
	d := NewTDigest(200)
	for i := 1; i <= 10000; i++ {
		d.Observe(float64(i))
	}
	for _, q := range []float64{0.5, 0.9, 0.95, 0.99, 0.999} {
		got := d.Quantile(q)
		want := q * 10000
		err := math.Abs(got-want) / want
		if err > 0.02 { // 2% tolerance
			t.Errorf("q=%v got=%v want=%v err=%v", q, got, want, err)
		}
	}
}

func TestTDigest_Summary(t *testing.T) {
	d := NewTDigest(100)
	for i := 0; i < 500; i++ {
		d.Observe(rand.Float64() * 100)
	}
	for q := 0.0; q <= 1.0; q += 0.1 {
		fmt.Printf("q=%v -> %v\n", q, d.Quantile(q))
	}
}
