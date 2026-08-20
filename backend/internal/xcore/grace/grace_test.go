package grace

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestShutdown_Order(t *testing.T) {
	m := New(time.Second)
	var calls []string
	m.Register(Hook{Name: "a", Fn: func(_ context.Context) error { calls = append(calls, "a"); return nil }})
	m.Register(Hook{Name: "b", Fn: func(_ context.Context) error { calls = append(calls, "b"); return nil }})
	if err := m.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "a" || calls[1] != "b" {
		t.Fatal("顺序错:", calls)
	}
}

func TestShutdown_Timeout(t *testing.T) {
	m := New(100 * time.Millisecond)
	m.Register(Hook{Name: "slow", Fn: func(_ context.Context) error {
		time.Sleep(500 * time.Millisecond)
		return nil
	}})
	err := m.Shutdown()
	if !errors.Is(err, ErrTimeout) {
		t.Fatal("应超时", err)
	}
}

func TestShutdown_Idempotent(t *testing.T) {
	m := New(time.Second)
	if err := m.Shutdown(); err != nil {
		t.Fatal(err)
	}
	// 二次调用应返回 ErrShutdown
	if err := m.Shutdown(); !errors.Is(err, ErrShutdown) {
		t.Fatal("二次应 ErrShutdown", err)
	}
}

func TestShutdown_ErrorCollect(t *testing.T) {
	m := New(time.Second)
	m.Register(Hook{Name: "x", Fn: func(_ context.Context) error { return errors.New("boom") }})
	err := m.Shutdown()
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatal("应包含 boom")
	}
	if !strings.Contains(err.Error(), "x") {
		t.Fatal("应包含 hook name")
	}
	if got := len(m.Errors()); got != 1 {
		t.Fatal("Errors 应记录")
	}
}

func TestShutdown_MultipleErrors(t *testing.T) {
	m := New(time.Second)
	m.Register(Hook{Name: "a", Fn: func(_ context.Context) error { return errors.New("err-a") }})
	m.Register(Hook{Name: "b", Fn: func(_ context.Context) error { return errors.New("err-b") }})
	err := m.Shutdown()
	if err == nil {
		t.Fatal("应返回错误")
	}
	errs := m.Errors()
	if len(errs) != 2 {
		t.Fatalf("应记录 2 个错误, got %d", len(errs))
	}
}

func TestShutdown_Panic(t *testing.T) {
	m := New(time.Second)
	m.Register(Hook{Name: "p", Fn: func(_ context.Context) error { panic("oops") }})
	err := m.Shutdown()
	if err == nil || !strings.Contains(err.Error(), "panic") {
		t.Fatal("应捕获 panic", err)
	}
}

func TestShutdown_ContinuesAfterError(t *testing.T) {
	m := New(time.Second)
	var calledB atomic.Bool
	m.Register(Hook{Name: "a", Fn: func(_ context.Context) error { return errors.New("err") }})
	m.Register(Hook{Name: "b", Fn: func(_ context.Context) error { calledB.Store(true); return nil }})
	m.Shutdown()
	if !calledB.Load() {
		t.Fatal("a 失败后 b 应仍执行")
	}
}

func TestShutdown_Elapsed(t *testing.T) {
	m := New(time.Second)
	m.Register(Hook{Name: "a", Fn: func(_ context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	}})
	m.Shutdown()
	if m.Elapsed() < 50*time.Millisecond {
		t.Fatal("elapsed 应 >= 50ms")
	}
}

func TestHookCount(t *testing.T) {
	m := New(time.Second)
	if m.HookCount() != 0 {
		t.Fatal("空")
	}
	m.Register(Hook{Name: "a", Fn: func(_ context.Context) error { return nil }})
	if m.HookCount() != 1 {
		t.Fatal("计数")
	}
	// nil fn 应忽略
	m.Register(Hook{Name: "b"})
	if m.HookCount() != 1 {
		t.Fatal("nil fn 应忽略")
	}
}

func TestRegisterWith(t *testing.T) {
	m := New(time.Second)
	m.RegisterWith("x", 10*time.Second, func(_ context.Context) error { return nil })
	if m.HookCount() != 1 {
		t.Fatal("RegisterWith")
	}
}

func TestOnHookError(t *testing.T) {
	m := New(time.Second)
	var captured atomic.Value
	m.OnHookError(func(name string, err error) {
		captured.Store(name + ":" + err.Error())
	})
	m.Register(Hook{Name: "x", Fn: func(_ context.Context) error { return errors.New("e1") }})
	m.Shutdown()
	s, _ := captured.Load().(string)
	if s != "x:e1" {
		t.Fatal("got", s)
	}
}

func TestShutdown_ConcurrentCall(t *testing.T) {
	m := New(time.Second)
	done := make(chan error, 2)
	go func() { done <- m.Shutdown() }()
	go func() { done <- m.Shutdown() }()
	e1 := <-done
	e2 := <-done
	// 应有 1 个 nil + 1 个 ErrShutdown（取决于时序）
	if e1 != nil && !errors.Is(e1, ErrShutdown) {
		t.Fatal("e1", e1)
	}
	if e2 != nil && !errors.Is(e2, ErrShutdown) {
		t.Fatal("e2", e2)
	}
}
