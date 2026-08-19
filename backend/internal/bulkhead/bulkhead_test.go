package bulkhead

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	b := New("x", 2)
	if err := b.Acquire(); err != nil {
		t.Fatal(err)
	}
	b.Release()
	if b.Stats().Current != 0 {
		t.Fatal("current")
	}
}

func TestAcquire_Full(t *testing.T) {
	b := New("x", 1)
	if err := b.Acquire(); err != nil {
		t.Fatal(err)
	}
	if err := b.Acquire(); err != ErrFull {
		t.Fatalf("expected ErrFull, got %v", err)
	}
	b.Release()
}

func TestRun(t *testing.T) {
	b := New("x", 1)
	err := b.Run(func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if b.Stats().Released != 1 {
		t.Fatal("released")
	}
}

func TestRun_PropagatesErr(t *testing.T) {
	b := New("x", 1)
	err := b.Run(func() error { return errors.New("boom") })
	if err == nil || err.Error() != "boom" {
		t.Fatal(err)
	}
}

func TestAcquireCtx_BlocksThenProceeds(t *testing.T) {
	b := New("x", 1)
	if err := b.Acquire(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := b.AcquireCtx(ctx); err != nil {
			t.Errorf("err: %v", err)
		}
		b.Release()
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	b.Release()
	<-done
}

func TestAcquireCtx_CancelWhileWaiting(t *testing.T) {
	b := New("x", 1)
	if err := b.Acquire(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.AcquireCtx(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
	if b.Stats().Rejected == 0 {
		t.Fatal("rejected counter")
	}
	b.Release()
}

func TestMax(t *testing.T) {
	b := New("x", 5)
	if b.Max() != 5 {
		t.Fatal(b.Max())
	}
	if b.Name() != "x" {
		t.Fatal(b.Name())
	}
}

func TestConcurrent(t *testing.T) {
	b := New("x", 4)
	var wg sync.WaitGroup
	var ran atomic.Uint64
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Run(func() error {
				ran.Add(1)
				return nil
			})
			if err != nil && !errors.Is(err, ErrFull) {
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()
	if ran.Load() == 0 {
		t.Fatal("no jobs ran")
	}
	s := b.Stats()
	if s.Released+s.Rejected != s.Acquired {
		t.Fatalf("counters mismatch: %+v", s)
	}
}
