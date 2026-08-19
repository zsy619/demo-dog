package timernx

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedule(t *testing.T) {
	w := New(20*time.Millisecond, 64)
	defer w.Stop()
	var n atomic.Int32
	w.Schedule(60*time.Millisecond, func() { n.Add(1) })
	time.Sleep(150 * time.Millisecond)
	if n.Load() == 0 {
		t.Fatal("应执行")
	}
}

func TestImmediate(t *testing.T) {
	w := New(20*time.Millisecond, 64)
	defer w.Stop()
	var n atomic.Int32
	w.Schedule(0, func() { n.Add(1) })
	time.Sleep(80 * time.Millisecond)
	if n.Load() == 0 {
		t.Fatal("立即应执行")
	}
}

func TestPanicRecover(t *testing.T) {
	w := New(20*time.Millisecond, 64)
	defer w.Stop()
	w.Schedule(0, func() { panic("x") })
	time.Sleep(80 * time.Millisecond)
}
