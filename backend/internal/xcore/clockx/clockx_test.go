package clockx

import (
	"testing"
	"time"
)

func TestReal(t *testing.T) {
	var c Clock = Real{}
	if c.Now().IsZero() {
		t.Fatal("now")
	}
}

func TestRealSince(t *testing.T) {
	var c Clock = Real{}
	a := c.Now()
	time.Sleep(10 * time.Millisecond)
	if c.Since(a) < 10*time.Millisecond {
		t.Fatal("since")
	}
}

func TestRealSleep(t *testing.T) {
	start := time.Now()
	var c Clock = Real{}
	c.Sleep(20 * time.Millisecond)
	if time.Since(start) < 20*time.Millisecond {
		t.Fatal("sleep")
	}
}

func TestFake(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(t0)
	f.Advance(time.Hour)
	if f.Now().Sub(t0) != time.Hour {
		t.Fatal("advance", f.Now())
	}
}

func TestFakeSleep(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(t0)
	f.Sleep(time.Minute)
	if f.Now().Sub(t0) != time.Minute {
		t.Fatal("sleep")
	}
}

func TestFakeSince(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(t0)
	f.Advance(time.Second)
	if f.Since(t0) != time.Second {
		t.Fatal("since")
	}
}

func TestFakeTicker(t *testing.T) {
	t0 := time.Now()
	f := NewFake(t0)
	tk := f.NewTicker(time.Millisecond)
	if tk == nil {
		t.Fatal("ticker")
	}
	tk.Stop()
}
