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

func TestDo(t *testing.T) {
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
