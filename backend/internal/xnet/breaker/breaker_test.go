package breaker

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestAcquire_Release(t *testing.T) {
	b := New(2)
	r1, ok := b.Acquire()
	if !ok {
		t.Fatal("应获取")
	}
	r2, ok := b.Acquire()
	if !ok {
		t.Fatal("应获取")
	}
	if _, ok := b.Acquire(); ok {
		t.Fatal("应失败")
	}
	r1()
	r2()
	if _, ok := b.Acquire(); !ok {
		t.Fatal("释放后应可用")
	}
}

func TestRun(t *testing.T) {
	b := New(1)
	if err := b.Run(func() error { return nil }); err != nil {
		t.Fatal("首次应成功")
	}
}

func TestRun_Overload(t *testing.T) {
	b := New(1)
	done := make(chan struct{})
	go func() {
		b.Run(func() error { <-done; return nil })
	}()
	// 等 goroutine 进入
	for b.Active() == 0 {
	}
	if err := b.Run(func() error { return nil }); !errors.Is(err, ErrOverload) {
		t.Fatal("应 ErrOverload")
	}
	close(done)
}

func TestRunCtx(t *testing.T) {
	b := New(1)
	if err := b.RunCtx(context.Background(), func(_ context.Context) error { return nil }); err != nil {
		t.Fatal("ctx")
	}
}

func TestStats(t *testing.T) {
	b := New(5)
	b.Run(func() error { return nil })
	b.Run(func() error { return nil })
	b.Acquire()
	s := b.Stats()
	if s.Total != 3 {
		t.Fatal("total")
	}
	b.Acquire() // 6th over limit
	if s.Fails != 0 {
		t.Fatal("fails")
	}
}

func TestSetLimit(t *testing.T) {
	b := New(1)
	b.SetLimit(10)
	if b.Stats().Limit != 10 {
		t.Fatal("setLimit")
	}
}

func TestConcurrent(t *testing.T) {
	b := New(10)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Run(func() error { return nil })
		}()
	}
	wg.Wait()
}
