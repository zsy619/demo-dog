package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestOnce(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()
	var n atomic.Int32
	s.Once("t", 20*time.Millisecond, func() { n.Add(1) })
	time.Sleep(100 * time.Millisecond)
	if n.Load() != 1 {
		t.Fatal("once", n.Load())
	}
}

func TestPeriodic(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()
	var n atomic.Int32
	s.Add("p", 20*time.Millisecond, func() { n.Add(1) })
	time.Sleep(120 * time.Millisecond)
	if n.Load() < 3 {
		t.Fatal("periodic", n.Load())
	}
}

func TestLen(t *testing.T) {
	s := New()
	s.Add("a", time.Second, func() {})
	if s.Len() != 1 {
		t.Fatal("len")
	}
}
