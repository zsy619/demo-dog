package metrics

import "testing"

func TestCounter(t *testing.T) {
	r := New()
	c := r.Counter("req")
	c.Add(5)
	c.Add(3)
	if c.Value() != 8 {
		t.Fatal("val")
	}
	if r.Counter("req") != c {
		t.Fatal("复用")
	}
}

func TestTotal(t *testing.T) {
	r := New()
	tot := r.Total("latency")
	tot.Observe(1)
	tot.Observe(3)
	if tot.Mean() != 2 {
		t.Fatal("mean")
	}
	if tot.Count() != 2 {
		t.Fatal("count")
	}
}

func TestHist(t *testing.T) {
	r := New()
	h := r.Hist("size")
	h.Observe(1)
	h.Observe(5)
	h.Observe(3)
	s := h.Snap()
	if s.Count != 3 || s.Min != 1 || s.Max != 5 {
		t.Fatal("snap")
	}
	if s.Mean != 3 {
		t.Fatal("mean")
	}
}

func TestHist_Empty(t *testing.T) {
	h := &Hist{}
	s := h.Snap()
	if s.Count != 0 {
		t.Fatal("empty")
	}
}
