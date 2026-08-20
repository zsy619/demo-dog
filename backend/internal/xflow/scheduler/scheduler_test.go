package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestOnce(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()
	var n atomic.Int32
	s.Once("t", 20*time.Millisecond, func(ctx context.Context) { n.Add(1) })
	time.Sleep(100 * time.Millisecond)
	if n.Load() != 1 {
		t.Fatal("once", n.Load())
	}
}

func TestPeriodic(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()
	var n atomic.Int32
	s.Every("p", 20*time.Millisecond, func(ctx context.Context) { n.Add(1) })
	time.Sleep(120 * time.Millisecond)
	if got := n.Load(); got < 3 {
		t.Fatal("periodic", got)
	}
}

func TestLen(t *testing.T) {
	s := New()
	s.Every("a", time.Second, func(ctx context.Context) {})
	if s.Len() != 1 {
		t.Fatal("len")
	}
}

func TestCancel(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()
	var n atomic.Int32
	s.Every("c", 20*time.Millisecond, func(ctx context.Context) { n.Add(1) })
	time.Sleep(50 * time.Millisecond)
	if !s.Cancel("c") {
		t.Fatal("cancel miss")
	}
	if s.Cancel("c") {
		t.Fatal("double cancel")
	}
}

func TestPanicRecover(t *testing.T) {
	s := New()
	var captured atomic.Value
	s.OnPanic(func(name string, err error) {
		captured.Store(err.Error())
	})
	s.Start()
	defer s.Stop()
	s.Once("p", 10*time.Millisecond, func(ctx context.Context) { panic("boom") })
	time.Sleep(100 * time.Millisecond)
	v, ok := captured.Load().(string); _ = v
	if !ok {
		t.Fatal("panic 未被捕获")
	}
	if v == "" {
		t.Fatal("empty panic msg")
	}
}

func TestStopIdempotent(t *testing.T) {
	s := New()
	s.Start()
	s.Stop()
	// 第二次调用应安全
	s.Stop()
}

func TestStartIdempotent(t *testing.T) {
	s := New()
	s.Start()
	s.Start() // 不应创建多个 loop
	s.Stop()
}

var _ = errors.New
