package throttle

import (
	"testing"
	"time"
)

func TestAllow(t *testing.T) {
	th := New(20 * time.Millisecond)
	if !th.Allow() {
		t.Fatal("first")
	}
	if th.Allow() {
		t.Fatal("second 应拒")
	}
}

func TestDo(t *testing.T) {
	th := New(20 * time.Millisecond)
	if !th.Do(func() {}) {
		t.Fatal("do")
	}
	if th.Do(func() {}) {
		t.Fatal("second")
	}
}

func TestWait(t *testing.T) {
	th := New(10 * time.Millisecond)
	start := time.Now()
	th.Wait()
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("wait")
	}
}

func TestWindow(t *testing.T) {
	th := New(time.Second)
	if th.Window() != time.Second {
		t.Fatal("window")
	}
}
