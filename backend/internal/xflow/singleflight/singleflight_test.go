package singleflight

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo(t *testing.T) {
	g := New[string, int]()
	v, err := g.Do("k", func() (int, error) { return 1, nil })
	if err != nil || v != 1 {
		t.Fatal("do")
	}
}

func TestMerge(t *testing.T) {
	g := New[string, int]()
	var n atomic.Int32
	var got int
	done := make(chan struct{})
	start := make(chan struct{})
	go func() {
		close(start)
		v, _ := g.Do("k", func() (int, error) {
			n.Add(1)
			time.Sleep(50 * time.Millisecond)
			return 42, nil
		})
		got = v
		close(done)
	}()
	<-start
	time.Sleep(5 * time.Millisecond)
	v, _ := g.Do("k", func() (int, error) {
		n.Add(1)
		return 0, nil
	})
	if v != 42 {
		t.Fatal("merge", v)
	}
	<-done
	if got != 42 {
		t.Fatal("got", got)
	}
	if n.Load() != 1 {
		t.Fatal("merge missed", n.Load())
	}
}

func TestForget(t *testing.T) {
	g := New[string, int]()
	g.Forget("k")
	if g.Inflight() != 0 {
		t.Fatal("inflight")
	}
}

func TestPanic(t *testing.T) {
	g := New[string, int]()
	v, err := g.Do("k", func() (int, error) { panic("boom") })
	if err == nil || v != 0 {
		t.Fatal("panic 应转为 error")
	}
}

func TestPanicShared(t *testing.T) {
	g := New[string, int]()
	var n atomic.Int32
	type result struct{ v int; err error }
	res := make(chan result, 2)
	start := make(chan struct{})
	go func() {
		close(start)
		v, err := g.Do("k", func() (int, error) {
			n.Add(1)
			time.Sleep(50 * time.Millisecond)
			panic("boom")
			return 0, nil
		})
		res <- result{v, err}
	}()
	<-start
	time.Sleep(5 * time.Millisecond)
	v, err := g.Do("k", func() (int, error) {
		n.Add(1)
		return 0, nil
	})
	res <- result{v, err}
	for i := 0; i < 2; i++ {
		r := <-res
		if r.err == nil {
			t.Fatal("共享 panic 应转为 error")
		}
	}
	if n.Load() != 1 {
		t.Fatal("应合并为 1 次调用")
	}
}

func TestDoCtx(t *testing.T) {
	g := New[string, int]()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v, err := g.DoCtx(ctx, "k", func() (int, error) { return 1, nil })
	if err == nil || v != 0 {
		t.Fatal("已 cancel ctx 应立即返回")
	}
}

func TestDoCtxDuringWait(t *testing.T) {
	g := New[string, int]()
	start := make(chan struct{})
	go func() {
		close(start)
		g.Do("k", func() (int, error) {
			time.Sleep(200 * time.Millisecond)
			return 1, nil
		})
	}()
	<-start
	time.Sleep(10 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := g.DoCtx(ctx, "k", func() (int, error) {
		t.Fatal("不应被调用")
		return 0, nil
	})
	if err == nil {
		t.Fatal("等待者应收到 ctx 错误")
	}
}
