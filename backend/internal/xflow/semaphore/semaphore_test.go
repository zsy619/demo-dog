package semaphore

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	s := NewWeighted(2)
	s.Acquire(context.Background(), 2)
	if s.Available() != 0 {
		t.Fatal("avail")
	}
	s.Release(2)
	if s.Available() != 2 {
		t.Fatal("avail after")
	}
}

func TestTryAcquire(t *testing.T) {
	s := NewWeighted(1)
	if !s.TryAcquire(1) {
		t.Fatal("first")
	}
	if s.TryAcquire(1) {
		t.Fatal("应拒")
	}
}

func TestWaitAndNotify(t *testing.T) {
	s := NewWeighted(1)
	s.Acquire(context.Background(), 1)
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		s.Acquire(ctx, 1)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	s.Release(1)
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("应唤醒")
	}
}

func TestAcquire_Cancel(t *testing.T) {
	s := NewWeighted(1)
	s.Acquire(context.Background(), 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Acquire(ctx, 1); !errors.Is(err, ErrExhausted) {
		t.Fatal("应 ErrExhausted")
	}
}

func TestDoExtended(t *testing.T) {
	s := NewWeighted(1)
	err := s.Do(context.Background(), 1, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
}

func TestAvailable(t *testing.T) {
	s := NewWeighted(3)
	if s.Available() != 3 {
		t.Fatal("avail")
	}
}

func TestMixedWeights(t *testing.T) {
	s := NewWeighted(5)
	// 占用 4，cur=4
	if err := s.Acquire(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	// waiter 1: 需 2
	go func() {
		s.Acquire(context.Background(), 2)
	}()
	time.Sleep(10 * time.Millisecond)
	// Release(3): cur=1, w(2): 1+2=3≤5, wake, cur=3
	// 可用 = 5 - 3 = 2
	s.Release(3)
	time.Sleep(50 * time.Millisecond)
	if s.Available() != 2 {
		t.Fatalf("avail should be 2, got %d", s.Available())
	}
	// 清理
	s.Release(1) // cur=2, 可用=3
	s.Release(2) // cur=0, 可用=5
}

func TestCtxCancel(t *testing.T) {
	s := NewWeighted(1)
	s.Acquire(context.Background(), 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Acquire(ctx, 1)
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, ErrExhausted) {
			t.Fatal("应 ErrExhausted", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("应取消")
	}
	if s.Waiters() != 0 {
		t.Fatal("waiter 应被移除")
	}
}

func TestWaiters(t *testing.T) {
	s := NewWeighted(1)
	s.Acquire(context.Background(), 1)
	go func() { s.Acquire(context.Background(), 1) }()
	go func() { s.Acquire(context.Background(), 1) }()
	time.Sleep(20 * time.Millisecond)
	if s.Waiters() != 2 {
		t.Fatal("应 2 个 waiter")
	}
}

func TestErrTooLarge(t *testing.T) {
	s := NewWeighted(2)
	if err := s.Acquire(context.Background(), 3); err != ErrTooLarge {
		t.Fatal("应 ErrTooLarge")
	}
}

func TestNilWeight(t *testing.T) {
	s := NewWeighted(1)
	if err := s.Acquire(context.Background(), 0); err != nil {
		t.Fatal("weight=0 应 no-op")
	}
	if !s.TryAcquire(0) {
		t.Fatal("weight=0 应 true")
	}
}

func TestOverRelease(t *testing.T) {
	s := NewWeighted(2)
	// 释放超过 cur
	s.Release(10)
	if s.Available() != 2 {
		t.Fatal("过释放不应导致 available < 0")
	}
}

func TestDoWithPanic(t *testing.T) {
	s := NewWeighted(1)
	err := s.Do(context.Background(), 1, func() error {
		if s.Available() != 0 {
			t.Fatal("应被占用")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.Available() != 1 {
		t.Fatal("Do 后应释放")
	}
}
