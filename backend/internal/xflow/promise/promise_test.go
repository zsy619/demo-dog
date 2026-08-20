package promise

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	p := New[int]()
	p.Resolve(42)
	v, _ := p.Await()
	if v != 42 {
		t.Fatal("v")
	}
}

func TestReject(t *testing.T) {
	p := New[int]()
	p.Reject(errors.New("x"))
	if _, err := p.Await(); err == nil {
		t.Fatal("err")
	}
}

func TestOnce(t *testing.T) {
	p := New[int]()
	p.Resolve(1)
	p.Resolve(2)
	v, _ := p.Await()
	if v != 1 {
		t.Fatal("once")
	}
}

func TestIsDone(t *testing.T) {
	p := New[int]()
	if p.IsDone() {
		t.Fatal("应未完成")
	}
	p.Resolve(1)
	if !p.IsDone() {
		t.Fatal("应完成")
	}
}

func TestRun(t *testing.T) {
	p := Run(func() (int, error) { time.Sleep(20 * time.Millisecond); return 7, nil })
	v, err := p.Await()
	if err != nil || v != 7 {
		t.Fatal("run")
	}
}

func TestAll(t *testing.T) {
	a := New[int]()
	b := New[int]()
	go func() { a.Resolve(1) }()
	go func() { b.Resolve(2) }()
	vs, err := All(a, b)
	if err != nil || len(vs) != 2 || vs[0] != 1 || vs[1] != 2 {
		t.Fatal("all")
	}
}

func TestPanic(t *testing.T) {
	p := Run(func() (int, error) { panic("boom") })
	_, err := p.Await()
	if err == nil {
		t.Fatal("panic 应转为 error")
	}
}

func TestAwaitCtx(t *testing.T) {
	p := New[int]()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.AwaitCtx(ctx)
	if err != context.Canceled {
		t.Fatal("应 Canceled")
	}
}

func TestResolveIdempotent(t *testing.T) {
	p := New[int]()
	p.Resolve(1)
	p.Resolve(2) // 应被忽略
	v, _ := p.Await()
	if v != 1 {
		t.Fatal("应=1", v)
	}
}

func TestAllCtxCancel(t *testing.T) {
	p1 := New[int]()
	p2 := New[int]()
	p1.Resolve(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := AllCtx(ctx, p1, p2)
	if err != context.Canceled {
		t.Fatal("应 Canceled")
	}
}

func TestAllNoLeak(t *testing.T) {
	// All 失败不应阻止其他 promise 完成
	done := make(chan struct{})
	p1 := New[int]()
	p2 := New[int]()
	go func() {
		p1.Resolve(1)
		p2.Reject(errors.New("err"))
		close(done)
	}()
	// 先 Resolved 1, 后 Reject 2
	time.Sleep(10 * time.Millisecond)
	out, err := All(p1, p2)
	if err == nil {
		t.Fatal("应返回错误")
	}
	if out[0] != 1 {
		t.Fatal("out[0] 应=1")
	}
	<-done
}
