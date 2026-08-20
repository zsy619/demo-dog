package executor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSubmit(t *testing.T) {
	e := New(2, 100)
	defer e.Close()
	var n atomic.Int32
	for i := 0; i < 50; i++ {
		e.Submit(func() { n.Add(1) })
	}
	time.Sleep(100 * time.Millisecond)
	if n.Load() != 50 {
		t.Fatal("missed", n.Load())
	}
}

func TestSubmitBlocking(t *testing.T) {
	e := New(2, 4)
	defer e.Close()
	for i := 0; i < 3; i++ {
		e.SubmitBlocking(func() {})
	}
	if e.QueueLen() < 0 {
		t.Fatal("queue")
	}
}

func TestCloseTwice(t *testing.T) {
	e := New(1, 1)
	e.Close()
	e.Close()
}

func TestPanicRecover(t *testing.T) {
	e := New(1, 8)
	defer e.Close()
	e.Submit(func() { panic("boom") })
	e.Submit(func() {})
	time.Sleep(100 * time.Millisecond)
	st := e.Stats()
	if st.Panics < 1 {
		t.Fatal("应记录 panic")
	}
	if st.Executed != 2 {
		t.Fatal("应执行 2 个任务（含 panic 后）")
	}
}

func TestSubmitCtx(t *testing.T) {
	e := New(1, 1)
	defer e.Close()
	// 先填满
	for i := 0; i < 8; i++ {
		e.Submit(func() { time.Sleep(50 * time.Millisecond) })
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e.SubmitCtx(ctx, func() {}) {
		t.Fatal("cancel ctx 应失败")
	}
}

func TestSubmitAfterClose(t *testing.T) {
	e := New(1, 8)
	e.Close()
	if e.Submit(func() {}) {
		t.Fatal("应失败")
	}
	if e.SubmitBlocking(func() {}) {
		t.Fatal("应失败")
	}
}

func TestNilJob(t *testing.T) {
	e := New(1, 8)
	defer e.Close()
	if e.Submit(nil) {
		t.Fatal("nil 应 false")
	}
	if e.SubmitBlocking(nil) {
		t.Fatal("nil 应 false")
	}
}

func TestQueueStats(t *testing.T) {
	e := New(1, 16)
	defer e.Close()
	for i := 0; i < 5; i++ {
		e.Submit(func() { time.Sleep(100 * time.Millisecond) })
	}
	time.Sleep(5 * time.Millisecond)
	if e.QueueLen() == 0 {
		t.Fatal("应有排队")
	}
}
