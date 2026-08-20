package timeoutlearner

import (
	"testing"
	"time"
)

func TestTimeout_Fallback(t *testing.T) {
	l := New(Config{Fallback: 500 * time.Millisecond})
	if got := l.Timeout("missing"); got != 500*time.Millisecond {
		t.Fatal("fallback")
	}
}

func TestTimeout_ClampMin(t *testing.T) {
	l := New(Config{Min: time.Second})
	l.Observe("a", 10*time.Millisecond)
	if got := l.Timeout("a"); got != time.Second {
		t.Fatal("min")
	}
}

func TestTimeout_ClampMax(t *testing.T) {
	l := New(Config{Max: time.Second})
	l.Observe("a", 10*time.Second)
	if got := l.Timeout("a"); got != time.Second {
		t.Fatal("max")
	}
}

func TestTimeout_Safety(t *testing.T) {
	l := New(Config{Safety: 2.0, Min: time.Millisecond, Max: time.Minute, Fallback: time.Millisecond})
	l.Observe("a", 200*time.Millisecond)
	if got := l.Timeout("a"); got != 400*time.Millisecond {
		t.Fatal("safety")
	}
}

func TestForget(t *testing.T) {
	l := New(Config{Fallback: time.Second})
	l.Observe("a", 100*time.Millisecond)
	l.Forget("a")
	if got := l.Timeout("a"); got != time.Second {
		t.Fatal("forget")
	}
}

func TestStats(t *testing.T) {
	l := New(Config{})
	l.Observe("a", 100*time.Millisecond)
	l.Observe("a", 200*time.Millisecond)
	s := l.Stats("a")
	if s.Count != 2 {
		t.Fatal("count")
	}
}

func TestSnapshot(t *testing.T) {
	l := New(Config{})
	l.Observe("a", 100*time.Millisecond)
	l.Observe("b", 200*time.Millisecond)
	snap := l.Snapshot()
	if len(snap) != 2 {
		t.Fatal("snap")
	}
}

func TestReset(t *testing.T) {
	l := New(Config{})
	l.Observe("a", 100*time.Millisecond)
	l.Reset()
	if l.Stats("a").Count != 0 {
		t.Fatal("reset")
	}
}

func TestSanity(t *testing.T) {
	if !Sanity() {
		t.Fatal("sanity")
	}
}
