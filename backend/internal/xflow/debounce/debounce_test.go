package debounce

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebounce_FiresOnce(t *testing.T) {
	var n atomic.Int32
	d := New(40*time.Millisecond, func() { n.Add(1) })
	for i := 0; i < 5; i++ {
		d.Trigger()
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if n.Load() != 1 {
		t.Fatal("应只执行一次")
	}
}

func TestDebounce_Pending(t *testing.T) {
	d := New(time.Second, func() {})
	d.Trigger()
	if !d.Pending() {
		t.Fatal("应有 pending")
	}
	d.Cancel()
	if d.Pending() {
		t.Fatal("Cancel 后无 pending")
	}
}

func TestDebounce_Flush(t *testing.T) {
	var n atomic.Int32
	d := New(time.Hour, func() { n.Add(1) })
	d.Trigger()
	d.Flush()
	if n.Load() != 1 {
		t.Fatal("Flush 应立即执行")
	}
}

func TestThrottle_FirstWins(t *testing.T) {
	var n atomic.Int32
	tr := NewThrottle(50*time.Millisecond, func() { n.Add(1) })
	if !tr.Try() {
		t.Fatal("首次应成功")
	}
	if tr.Try() {
		t.Fatal("窗口内应拒")
	}
	time.Sleep(80 * time.Millisecond)
	if !tr.Try() {
		t.Fatal("窗口过后应成功")
	}
	if n.Load() != 2 {
		t.Fatal("count")
	}
}

func TestThrottle_Reset(t *testing.T) {
	tr := NewThrottle(time.Second, func() {})
	tr.Reset()
	if !tr.Try() {
		t.Fatal("reset 后应可用")
	}
}
