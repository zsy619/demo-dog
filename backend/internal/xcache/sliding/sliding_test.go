package sliding

import (
	"testing"
	"time"
)

func newLimiter() *Limiter {
	return New(time.Second, 3).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestAllow(t *testing.T) {
	l := newLimiter()
	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatal("应允许")
		}
	}
	if l.Allow("k") {
		t.Fatal("第 4 次应拒绝")
	}
}

func TestAllow_DifferentKeys(t *testing.T) {
	l := newLimiter()
	for i := 0; i < 3; i++ {
		l.Allow("a")
	}
	if !l.Allow("b") {
		t.Fatal("不同 key 独立计数")
	}
}

func TestWindow_Slide(t *testing.T) {
	now := time.Unix(1700000000, 0)
	l := New(time.Second, 2).WithTime(func() time.Time { return now })
	l.Allow("k")
	l.Allow("k")
	if l.Allow("k") {
		t.Fatal("应拒绝")
	}
	now = now.Add(2 * time.Second)
	if !l.Allow("k") {
		t.Fatal("窗口过后应允许")
	}
}

func TestCount(t *testing.T) {
	l := newLimiter()
	l.Allow("k")
	l.Allow("k")
	if l.Count("k") != 2 {
		t.Fatal("count")
	}
	if l.Count("missing") != 0 {
		t.Fatal("missing")
	}
}

func TestReset(t *testing.T) {
	l := newLimiter()
	l.Allow("k")
	l.Allow("k")
	l.Reset("k")
	if l.Count("k") != 0 {
		t.Fatal("reset")
	}
}

func TestCleanup(t *testing.T) {
	now := time.Unix(1700000000, 0)
	l := New(time.Second, 3).WithTime(func() time.Time { return now })
	l.Allow("a")
	l.Allow("b")
	now = now.Add(2 * time.Second)
	if n := l.Cleanup(); n != 2 {
		t.Fatal("cleanup")
	}
}
