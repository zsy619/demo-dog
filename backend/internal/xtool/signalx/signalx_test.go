package signalx

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	s := New(2)
	s.Acquire()
	s.Acquire()
	if s.InUse() != 2 {
		t.Fatal("inuse")
	}
	if s.Capacity() != 2 {
		t.Fatal("cap")
	}
	s.Release()
	if s.InUse() != 1 {
		t.Fatal("release")
	}
}

func TestTryAcquire(t *testing.T) {
	s := New(1)
	if !s.TryAcquire() {
		t.Fatal("first")
	}
	if s.TryAcquire() {
		t.Fatal("second")
	}
	s.Release()
	if !s.TryAcquire() {
		t.Fatal("third")
	}
	s.Release()
}

func TestAcquireCtx_Cancel(t *testing.T) {
	s := New(1)
	s.Acquire()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.AcquireCtx(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("应取消")
	}
}

func TestAcquireCtx_OK(t *testing.T) {
	s := New(1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.AcquireCtx(ctx); err != nil {
		t.Fatal("应成功")
	}
	s.Release()
}

func TestDo(t *testing.T) {
	s := New(1)
	s.Do(func() {})
	if s.InUse() != 0 {
		t.Fatal("应放回")
	}
}

func TestDoCtx(t *testing.T) {
	s := New(1)
	err := s.DoCtx(context.Background(), func() {})
	if err != nil {
		t.Fatal("应成功")
	}
}

func TestConcurrent(t *testing.T) {
	s := New(5)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Do(func() { time.Sleep(5 * time.Millisecond) })
		}()
	}
	wg.Wait()
}
