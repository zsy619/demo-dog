package sampler

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestShouldKeep_AtFullRate(t *testing.T) {
	c := New(Default())
	for i := 0; i < 1000; i++ {
		if !c.ShouldKeep() {
			t.Fatal("at full rate everything should be kept")
		}
	}
}

func TestShouldKeep_LowRate(t *testing.T) {
	c := New(Config{Rate: 0.0, MinRate: 0.0, MaxRate: 1.0, Window: 100000})
	kept := 0
	for i := 0; i < 100; i++ {
		if c.ShouldKeep() {
			kept++
		}
	}
	if kept > 5 {
		t.Fatalf("expected near-zero keep at rate=0, got %d", kept)
	}
}

func TestObserve_NoError(t *testing.T) {
	c := New(Default())
	for i := 0; i < 1000; i++ {
		c.Observe(false)
	}
	if c.curErr.Load() != 0 {
		t.Fatal("err counter")
	}
}

func TestObserve_AdjustsOnErrors(t *testing.T) {
	c := New(Config{Rate: 1.0, MinRate: 0.01, MaxRate: 1.0, TargetEPS: 10000, Window: 100})
	for i := 0; i < 100; i++ {
		c.Observe(true)
	}
	r := c.Rate()
	if r >= 1.0 {
		t.Fatalf("rate should drop on errors, got %v", r)
	}
}

func TestSetRate(t *testing.T) {
	c := New(Default())
	c.SetRate(0.5)
	if c.Rate() != 0.5 {
		t.Fatal("set rate")
	}
	c.SetRate(2.0) // clamp to max
	if c.Rate() != 1.0 {
		t.Fatal("clamp high")
	}
	c.SetRate(-1.0) // clamp to min
	if c.Rate() != 0.01 {
		t.Fatal("clamp low")
	}
}

func TestStats(t *testing.T) {
	c := New(Default())
	for i := 0; i < 100; i++ {
		c.ShouldKeep()
		c.Observe(i%5 == 0)
	}
	s := c.Stats()
	if s.Decisions != 200 {
		t.Fatalf("decisions: %d", s.Decisions)
	}
	if s.Errors != 20 {
		t.Fatalf("errors: %d", s.Errors)
	}
	if s.KeptRatio < 0.4 {
		t.Fatalf("kept ratio low: %v", s.KeptRatio)
	}
}

func TestClamp(t *testing.T) {
	if clamp(-1, 0, 1) != 0 {
		t.Fatal("clamp low")
	}
	if clamp(2, 0, 1) != 1 {
		t.Fatal("clamp high")
	}
	if clamp(0.5, 0, 1) != 0.5 {
		t.Fatal("clamp mid")
	}
}

func TestConcurrent_Atomic(t *testing.T) {
	c := New(Default())
	var kept atomic.Uint64
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if c.ShouldKeep() {
					kept.Add(1)
				}
				c.Observe(false)
			}
		}()
	}
	wg.Wait()
	s := c.Stats()
	if s.Decisions != 2000 {
		t.Fatalf("decisions: %d", s.Decisions)
	}
	if kept.Load() == 0 {
		t.Fatal("expected some keeps")
	}
}
