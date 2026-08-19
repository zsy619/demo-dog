package rwqueue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPushPop(t *testing.T) {
	q := New[int](4)
	q.Push(1)
	v, ok := q.Pop()
	if !ok || v != 1 {
		t.Fatal("pp")
	}
}

func TestTryPush(t *testing.T) {
	q := New[int](1)
	if !q.TryPush(1) {
		t.Fatal("first")
	}
	if q.TryPush(2) {
		t.Fatal("应满")
	}
}

func TestTryPop(t *testing.T) {
	q := New[int](2)
	if _, ok := q.TryPop(); ok {
		t.Fatal("应空")
	}
}

func TestPushCtx(t *testing.T) {
	q := New[int](1)
	q.Push(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := q.PushCtx(ctx, 2); !errors.Is(err, context.Canceled) {
		t.Fatal("ctx", err)
	}
}

func TestPopCtx_Empty(t *testing.T) {
	q := New[int](2)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := q.PopCtx(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("timeout", err)
	}
}

func TestClose(t *testing.T) {
	q := New[int](1)
	q.Push(1)
	q.Close()
	v, ok := q.Pop()
	if !ok || v != 1 {
		t.Fatal("after close", ok, v)
	}
}

func TestLenCap(t *testing.T) {
	q := New[int](5)
	if q.Cap() != 5 || q.Len() != 0 {
		t.Fatal("size")
	}
}
