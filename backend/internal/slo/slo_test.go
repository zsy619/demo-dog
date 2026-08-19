package slo

import (
	"sync"
	"testing"
	"time"
)

func newTracker() *Tracker {
	return NewTracker().WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestSLO_Validate(t *testing.T) {
	cases := []struct {
		name string
		s    SLO
		err  bool
	}{
		{"valid", SLO{Name: "x", Target: 0.999, Window: time.Minute}, false},
		{"no name", SLO{Target: 0.999, Window: time.Minute}, true},
		{"bad target high", SLO{Name: "x", Target: 1.5, Window: time.Minute}, true},
		{"bad target zero", SLO{Name: "x", Target: 0, Window: time.Minute}, true},
		{"bad target negative", SLO{Name: "x", Target: -0.1, Window: time.Minute}, true},
		{"bad window", SLO{Name: "x", Target: 0.99, Window: 0}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.s.Validate()
			if (err != nil) != c.err {
				t.Fatalf("err=%v want %v", err, c.err)
			}
		})
	}
}

func TestTracker_RegisterDuplicate(t *testing.T) {
	tr := newTracker()
	if err := tr.Register(&SLO{Name: "x", Target: 0.99, Window: time.Minute}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Register(&SLO{Name: "x", Target: 0.95, Window: time.Minute}); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestTracker_Compute_AllSuccess(t *testing.T) {
	tr := newTracker()
	tr.MustRegister(&SLO{Name: "x", Target: 0.99, Window: time.Minute})
	for i := 0; i < 100; i++ {
		tr.Record("x", Sample{Success: true, Took: 10 * time.Millisecond})
	}
	s, ok := tr.Compute("x")
	if !ok {
		t.Fatal("missing")
	}
	if s.Ratio != 1.0 || !s.Healthy {
		t.Fatal(s)
	}
	if s.Samples != 100 || s.Successes != 100 {
		t.Fatal("counts")
	}
}

func TestTracker_Compute_BelowTarget(t *testing.T) {
	tr := newTracker()
	tr.MustRegister(&SLO{Name: "x", Target: 0.99, Window: time.Minute})
	for i := 0; i < 100; i++ {
		ok := i < 95
		tr.Record("x", Sample{Success: ok, Took: time.Millisecond})
	}
	s, _ := tr.Compute("x")
	if s.Healthy {
		t.Fatal("should be unhealthy")
	}
	if s.Ratio != 0.95 {
		t.Fatalf("ratio: %v", s.Ratio)
	}
	if tr.Alerts() == 0 {
		t.Fatal("alerts counter")
	}
}

func TestTracker_Percentiles(t *testing.T) {
	tr := newTracker()
	tr.MustRegister(&SLO{Name: "x", Target: 0.5, Window: time.Hour})
	for i := 1; i <= 100; i++ {
		tr.Record("x", Sample{Success: true, Took: time.Duration(i) * time.Millisecond})
	}
	s, _ := tr.Compute("x")
	if s.P50 < time.Millisecond || s.P50 > 100*time.Millisecond {
		t.Fatalf("p50: %v", s.P50)
	}
	if s.P95 <= s.P50 {
		t.Fatalf("p95 (%v) <= p50 (%v)", s.P95, s.P50)
	}
	if s.P99 <= s.P95 {
		t.Fatalf("p99 (%v) <= p95 (%v)", s.P99, s.P95)
	}
}

func TestTracker_WindowEviction(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tr := NewTracker().WithTime(func() time.Time { return now })
	tr.MustRegister(&SLO{Name: "x", Target: 0.5, Window: time.Minute})
	for i := 0; i < 50; i++ {
		tr.Record("x", Sample{Success: true, Took: time.Millisecond})
	}
	now = now.Add(2 * time.Minute)
	s, _ := tr.Compute("x")
	if s.Samples != 0 {
		t.Fatalf("samples: %d", s.Samples)
	}
}

func TestTracker_Compute_Unknown(t *testing.T) {
	tr := newTracker()
	if _, ok := tr.Compute("missing"); ok {
		t.Fatal("expected miss")
	}
}

func TestTracker_Snapshot(t *testing.T) {
	tr := newTracker()
	tr.MustRegister(&SLO{Name: "b", Target: 0.5, Window: time.Minute})
	tr.MustRegister(&SLO{Name: "a", Target: 0.5, Window: time.Minute})
	snap := tr.Snapshot()
	if len(snap) != 2 || snap[0].Name != "a" {
		t.Fatal("snapshot order")
	}
}

func TestTracker_ConcurrentRecord(t *testing.T) {
	tr := newTracker()
	tr.MustRegister(&SLO{Name: "x", Target: 0.5, Window: time.Hour})
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				tr.Record("x", Sample{Success: true, Took: time.Millisecond})
			}
		}()
	}
	wg.Wait()
	s, _ := tr.Compute("x")
	if s.Samples != 1000 {
		t.Fatalf("samples: %d", s.Samples)
	}
}
