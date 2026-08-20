package poolg

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSubmit(t *testing.T) {
	p := New(2, 8)
	defer p.Wait()
	var n atomic.Int32
	for i := 0; i < 10; i++ {
		if err := p.Submit(func() { n.Add(1) }); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(50 * time.Millisecond)
	if n.Load() != 10 {
		t.Fatal("submits")
	}
}

func TestSubmitCtx(t *testing.T) {
	p := New(1, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// 先填满队列
	for i := 0; i < 8; i++ {
		p.Submit(func() { time.Sleep(10 * time.Millisecond) })
	}
	cancel()
	if err := p.SubmitCtx(ctx, func() {}); err == nil {
		t.Fatal("已 cancel ctx 应失败")
	}
	p.Wait()
}

func TestTrySubmit(t *testing.T) {
	p := New(1, 1)
	defer p.Wait()
	if err := p.TrySubmit(func() {}); err != nil {
		t.Fatal(err)
	}
	if err := p.TrySubmit(func() {}); err != ErrQueueFull {
		t.Fatal("应 ErrQueueFull")
	}
}

func TestSubmitAfterClosed(t *testing.T) {
	p := New(1, 1)
	p.Wait()
	if err := p.Submit(func() {}); err != ErrClosed {
		t.Fatal("应 ErrClosed")
	}
}

func TestPanicRecover(t *testing.T) {
	p := New(1, 8)
	var captured atomic.Value
	p.OnPanic(func(name string, err error) {
		captured.Store(err.Error())
	})
	if err := p.Submit(func() { panic("oops") }); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	p.Wait()
	if _, ok := captured.Load().(string); !ok {
		t.Fatal("panic 未被捕获")
	}
	if p.Stats().Panics != 1 {
		t.Fatal("Panics 计数应=1")
	}
}

func TestConcurrent(t *testing.T) {
	p := New(4, 64)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Submit(func() {})
		}()
	}
	wg.Wait()
	p.Wait()
	s := p.Stats()
	if s.Submits != 100 {
		t.Fatal("stats", s)
	}
	if s.Executed != 100 {
		t.Fatal("executed", s)
	}
}

func TestNilTask(t *testing.T) {
	p := New(1, 1)
	defer p.Wait()
	if err := p.Submit(nil); err != nil {
		t.Fatal(err)
	}
	if err := p.TrySubmit(nil); err != nil {
		t.Fatal(err)
	}
	if err := p.SubmitCtx(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestWaitIdempotent(t *testing.T) {
	p := New(2, 4)
	p.Wait()
	p.Wait()  // 幂等
	p.Close() // 幂等
}

func TestStatsCounters(t *testing.T) {
	p := New(1, 1)
	defer p.Wait()
	// 填满 worker + queue
	block := make(chan struct{})
	p.Submit(func() { <-block }) // worker 正在执行
	time.Sleep(10 * time.Millisecond)
	// 此时 queue 容量=1，应能 TrySubmit 入队
	if err := p.TrySubmit(func() {}); err != nil {
		close(block)
		t.Fatal("第 2 个应入队", err)
	}
	// 此时 queue 已满
	if err := p.TrySubmit(func() {}); err != ErrQueueFull {
		close(block)
		t.Fatal("第 3 个应 ErrQueueFull", err)
	}
	close(block)
	st := p.Stats()
	if st.Submits != 2 || st.Dropped != 1 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestNilErrorCheck(t *testing.T) {
	// 确保错误变量非 nil
	if errors.Is(nil, ErrClosed) {
		t.Fatal("errors.Is(nil, ErrClosed) 应为 false")
	}
}
