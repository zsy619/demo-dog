package recurring

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTrigger(t *testing.T) {
	var n atomic.Int32
	r := New(20*time.Millisecond, func(_ int64) { n.Add(1) })
	r.Start()
	time.Sleep(120 * time.Millisecond)
	r.Stop()
	if n.Load() < 3 {
		t.Fatal("tick", n.Load())
	}
}

func TestDoubleStart(t *testing.T) {
	r := New(50*time.Millisecond, func(_ int64) {})
	r.Start()
	r.Start()
	time.Sleep(10 * time.Millisecond)
	r.Stop()
}

func TestDoubleStop(t *testing.T) {
	r := New(50*time.Millisecond, func(_ int64) {})
	r.Stop() // 应不 panic
}

func TestTick(t *testing.T) {
	r := New(20*time.Millisecond, func(_ int64) {})
	r.Start()
	time.Sleep(80 * time.Millisecond)
	r.Stop()
	if r.Tick() < 2 {
		t.Fatal("tick c", r.Tick())
	}
}
