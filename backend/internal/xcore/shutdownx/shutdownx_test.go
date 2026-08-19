package shutdownx

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestShutdown_LIFO(t *testing.T) {
	m := New()
	order := []string{}
	m.Register("a", 0, func(_ context.Context) error { order = append(order, "a"); return nil })
	m.Register("b", 0, func(_ context.Context) error { order = append(order, "b"); return nil })
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "b" || order[1] != "a" {
		t.Fatal("lifo", order)
	}
}

func TestShutdown_Error(t *testing.T) {
	m := New()
	m.Register("a", 0, func(_ context.Context) error { return errors.New("x") })
	err := m.Shutdown(context.Background())
	if err == nil {
		t.Fatal("应报错")
	}
}

func TestShutdown_Timeout(t *testing.T) {
	m := New()
	m.Register("a", 10*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	err := m.Shutdown(context.Background())
	if err == nil {
		t.Fatal("应 timeout")
	}
}

func TestHookCount(t *testing.T) {
	m := New()
	if m.HookCount() != 0 {
		t.Fatal("empty")
	}
	m.Register("a", 0, func(_ context.Context) error { return nil })
	if m.HookCount() != 1 {
		t.Fatal("count")
	}
	m.Shutdown(context.Background())
	if m.HookCount() != 0 {
		t.Fatal("after")
	}
}
