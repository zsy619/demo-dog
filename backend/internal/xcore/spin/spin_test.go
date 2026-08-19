package spin

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestUntil_True(t *testing.T) {
	if !Until(time.Now().Add(time.Second), func() bool { return true }) {
		t.Fatal("true")
	}
}

func TestUntil_Timeout(t *testing.T) {
	if Until(time.Now().Add(20*time.Millisecond), func() bool { return false }) {
		t.Fatal("应超时")
	}
}

func TestNanos(t *testing.T) {
	n := 0
	Nanos(10, func() { n++ })
	if n != 10 {
		t.Fatal("nanos")
	}
}

func TestBackoff(t *testing.T) {
	b := NewBackoff()
	hit := false
	ok := b.Do(func() bool {
		hit = true
		return false
	})
	if ok {
		t.Fatal("应 false")
	}
	if !hit {
		t.Fatal("应执行")
	}
	b.Reset()
}

func TestBackoff_Success(t *testing.T) {
	b := NewBackoff()
	if !b.Do(func() bool { return true }) {
		t.Fatal("true")
	}
}

func TestWaitFor(t *testing.T) {
	n := atomic.Int32{}
	if WaitFor(50*time.Millisecond, func() bool { return n.Load() > 0 }) {
		t.Fatal("初始应 false")
	}
	n.Add(1)
	if !WaitFor(50*time.Millisecond, func() bool { return n.Load() > 0 }) {
		t.Fatal("再次应 true")
	}
}

func TestPollingFlag(t *testing.T) {
	f := &PollingFlag{}
	f.Set(true)
	if !f.WaitUntilTrue(50 * time.Millisecond) {
		t.Fatal("flag")
	}
}
