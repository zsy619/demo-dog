package poolg

import (
	"context"
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
		p.Submit(func() { n.Add(1) })
	}
	time.Sleep(50 * time.Millisecond)
	if n.Load() != 10 {
		t.Fatal("submits")
	}
}

func TestSubmitCtx(t *testing.T) {
	p := New(1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !p.SubmitCtx(ctx, func() {}) {
		t.Fatal("应通过")
	}
	cancel()
	p.SubmitCtx(ctx, func() {})
}

func TestPanicRecover(t *testing.T) {
	p := New(1, 1)
	p.Submit(func() { panic("oops") })
	p.Submit(func() {})
	time.Sleep(50 * time.Millisecond)
	p.Wait()
}

func TestConcurrent(t *testing.T) {
	p := New(4, 16)
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
		t.Fatal("stats")
	}
}
