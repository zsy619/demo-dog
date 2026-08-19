package grace

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestShutdown_Order(t *testing.T) {
	m := New(time.Second)
	calls := []string{}
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
		t.Fatal("应超时")
	}
}

func TestShutdown_Idempotent(t *testing.T) {
	m := New(time.Second)
	m.Shutdown()
	if err := m.Shutdown(); err != nil {
		t.Fatal("二次应静默")
	}
}

func TestShutdown_ErrorProp(t *testing.T) {
	m := New(time.Second)
	m.Register(Hook{Name: "x", Fn: func(_ context.Context) error { return errors.New("boom") }})
	if err := m.Shutdown(); err == nil || err.Error() != "boom" {
		t.Fatal("应返回错误")
	}
}

func TestShutdown_Panic(t *testing.T) {
	m := New(time.Second)
	m.Register(Hook{Name: "p", Fn: func(_ context.Context) error { panic("oops") }})
	if err := m.Shutdown(); err == nil || err.Error() != "panic" {
		t.Fatal("应捕获 panic")
	}
}

func TestHookCount(t *testing.T) {
	m := New(time.Second)
	if m.HookCount() != 0 {
		t.Fatal("空")
	}
	m.Register(Hook{Name: "a"})
	if m.HookCount() != 1 {
		t.Fatal("计数")
	}
}
