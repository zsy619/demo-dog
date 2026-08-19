package circuit

import (
	"errors"
	"testing"
	"time"
)

func TestClosedToOpen(t *testing.T) {
	b := New(Config{FailureThreshold: 3, OpenFor: 50 * time.Millisecond})
	fn := func() error { return errors.New("boom") }
	for i := 0; i < 3; i++ {
		b.Call(fn)
	}
	if b.State() != StateOpen {
		t.Fatal("应打开")
	}
}

func TestOpenDenies(t *testing.T) {
	b := New(Config{FailureThreshold: 1, OpenFor: time.Second})
	b.Call(func() error { return errors.New("x") })
	if err := b.Call(func() error { return nil }); !errors.Is(err, ErrOpen) {
		t.Fatal("应 ErrOpen")
	}
}

func TestOpenToHalfOpen(t *testing.T) {
	b := New(Config{FailureThreshold: 1, OpenFor: 30 * time.Millisecond})
	b.Call(func() error { return errors.New("x") })
	time.Sleep(60 * time.Millisecond)
	if err := b.Call(func() error { return nil }); err != nil {
		t.Fatal("半开应允许")
	}
	if b.State() != StateClosed {
		t.Fatal("成功后应关闭")
	}
}

func TestHalfOpenFailureReopens(t *testing.T) {
	b := New(Config{FailureThreshold: 1, OpenFor: 30 * time.Millisecond})
	b.Call(func() error { return errors.New("x") })
	time.Sleep(60 * time.Millisecond)
	b.Call(func() error { return errors.New("x") })
	if b.State() != StateOpen {
		t.Fatal("半开失败应回到打开")
	}
}

func TestReset(t *testing.T) {
	b := New(Config{FailureThreshold: 1, OpenFor: time.Second})
	b.Call(func() error { return errors.New("x") })
	b.Reset()
	if b.State() != StateClosed {
		t.Fatal("reset")
	}
}

func TestStats(t *testing.T) {
	b := New(Config{FailureThreshold: 2, OpenFor: time.Second})
	b.Call(func() error { return nil })
	b.Call(func() error { return errors.New("x") })
	s := b.Stats()
	if s.Calls != 2 || s.Failed != 1 {
		t.Fatal("stats")
	}
}
