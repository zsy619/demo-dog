package procstate

import (
	"testing"
	"time"
)

func TestSnapshot(t *testing.T) {
	r := New()
	s := r.Snapshot()
	if s.GoVersion == "" {
		t.Fatal("版本")
	}
	if s.NumGoroutine <= 0 {
		t.Fatal("协程数")
	}
	if s.Uptime < 0 {
		t.Fatal("uptime")
	}
}

func TestIncCounter(t *testing.T) {
	r := New()
	r.IncCounter("hits")
	r.IncCounter("hits")
	r.IncCounter("misses")
	if r.CounterValue("hits") != 2 {
		t.Fatal("hits")
	}
	if r.CounterValue("misses") != 1 {
		t.Fatal("misses")
	}
}

func TestSetCounter(t *testing.T) {
	r := New()
	r.SetCounter("q", 42)
	if r.CounterValue("q") != 42 {
		t.Fatal("set")
	}
}

func TestNames(t *testing.T) {
	r := New()
	r.IncCounter("a")
	r.IncCounter("b")
	if len(r.CounterNames()) != 2 {
		t.Fatal("names")
	}
}

func TestReset(t *testing.T) {
	r := New()
	r.IncCounter("x")
	r.Reset()
	if len(r.CounterNames()) != 0 {
		t.Fatal("reset")
	}
}

func TestUptime_Grows(t *testing.T) {
	r := New()
	s1 := r.Snapshot()
	time.Sleep(20 * time.Millisecond)
	s2 := r.Snapshot()
	if s2.Uptime <= s1.Uptime {
		t.Fatal("uptime 应增长")
	}
}
