package bloom

import (
	"fmt"
	"testing"
)

func TestAddContains(t *testing.T) {
	f := New(1000, 0.01)
	f.Add([]byte("alpha"))
	if !f.Contains([]byte("alpha")) {
		t.Fatal("should contain")
	}
	if f.Contains([]byte("beta")) {
		t.Fatal("should not contain")
	}
}

func TestFalsePositiveRate(t *testing.T) {
	f := New(10000, 0.01)
	for i := 0; i < 10000; i++ {
		f.Add([]byte(fmt.Sprintf("item-%d", i)))
	}
	fps := 0
	trials := 10000
	for i := 0; i < trials; i++ {
		if f.Contains([]byte(fmt.Sprintf("miss-%d", i))) {
			fps++
		}
	}
	rate := float64(fps) / float64(trials)
	if rate > 0.05 {
		t.Fatalf("fp rate too high: %v", rate)
	}
}

func TestCount(t *testing.T) {
	f := New(100, 0.01)
	for i := 0; i < 5; i++ {
		f.Add([]byte(fmt.Sprintf("k-%d", i)))
	}
	if f.Count() != 5 {
		t.Fatal("count")
	}
}

func TestSize(t *testing.T) {
	f := New(100, 0.01)
	if f.Size() == 0 {
		t.Fatal("size 0")
	}
	if f.HashN() == 0 {
		t.Fatal("hashN 0")
	}
}

func TestEstimatedFPRate(t *testing.T) {
	f := New(100, 0.01)
	if f.EstimatedFPRate() != 0 {
		t.Fatal("empty should be 0")
	}
	for i := 0; i < 100; i++ {
		f.Add([]byte(fmt.Sprintf("k-%d", i)))
	}
	if f.EstimatedFPRate() <= 0 {
		t.Fatal("should be > 0")
	}
}

func TestNew_Defaults(t *testing.T) {
	f := New(0, 0)
	if f.Size() == 0 {
		t.Fatal("size")
	}
	f2 := New(0, 2)
	if f2.Size() == 0 {
		t.Fatal("size 2")
	}
}

func TestOptimalK(t *testing.T) {
	k := optimalK(1000, 10000)
	if k < 1 {
		t.Fatal("k")
	}
}

func TestOptimalM(t *testing.T) {
	m := optimalM(1000, 0.01)
	if m < 1 {
		t.Fatal("m")
	}
}

func TestHashAt_Diverse(t *testing.T) {
	h1 := hashAt([]byte("a"), 0)
	h2 := hashAt([]byte("a"), 1)
	if h1 == h2 {
		t.Fatal("different index should differ")
	}
	h3 := hashAt([]byte("b"), 0)
	if h1 == h3 {
		t.Fatal("different data should differ")
	}
}
