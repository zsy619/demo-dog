package metricsx

import (
	"sync"
	"testing"
)

func TestCounter(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("hits")
	c.Inc()
	c.Inc()
	c.Add(10)
	if c.Value() != 12 {
		t.Fatal("counter")
	}
	if r.Snapshot().Counters["hits"] != 12 {
		t.Fatal("snap")
	}
}

func TestGauge(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("temp")
	g.Set(3.14)
	if g.Value() < 3.13 || g.Value() > 3.15 {
		t.Fatal("gauge")
	}
}

func TestHistogram(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("lat", []float64{1, 5, 10})
	h.Observe(0.5)
	h.Observe(2)
	h.Observe(20)
	s := h.Stats()
	if s.Count != 3 {
		t.Fatal("count")
	}
}

func TestNames(t *testing.T) {
	r := NewRegistry()
	r.Counter("a")
	r.Gauge("b")
	r.Histogram("c", []float64{1})
	if len(r.Names()) != 3 {
		t.Fatal("names")
	}
}

func TestReset(t *testing.T) {
	r := NewRegistry()
	r.Counter("a").Inc()
	r.Reset()
	if len(r.Names()) != 0 {
		t.Fatal("reset")
	}
}

func TestConcurrent(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("x")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}
	wg.Wait()
	if c.Value() != 100 {
		t.Fatal("并发")
	}
}
