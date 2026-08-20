package prioq

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestQueue_PushPop(t *testing.T) {
	q := New()
	q.Push("low", 5)
	q.Push("high", 1)
	q.Push("mid", 3)
	if v, _ := q.Pop(); v != "high" {
		t.Fatal("应先弹出 high")
	}
	if v, _ := q.Pop(); v != "mid" {
		t.Fatal("应弹出 mid")
	}
}

func TestQueue_Len(t *testing.T) {
	q := New()
	if q.Len() != 0 {
		t.Fatal("空队列")
	}
	q.Push(1, 1)
	if q.Len() != 1 {
		t.Fatal("len")
	}
}

func TestQueue_Empty(t *testing.T) {
	q := New()
	if _, ok := q.Pop(); ok {
		t.Fatal("空不应有元素")
	}
}

func TestBatch_DrainOnSize(t *testing.T) {
	var received atomic.Int64
	b := NewBatch(3, time.Second, func(items []any) {
		received.Add(int64(len(items)))
	})
	defer b.Close()
	for i := 0; i < 5; i++ {
		b.Submit(i, int64(i))
	}
	for i := 0; i < 100 && received.Load() < 5; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if received.Load() != 5 {
		t.Fatal("应全部处理")
	}
}

func TestBatch_DrainOnTime(t *testing.T) {
	var received atomic.Int64
	b := NewBatch(100, 50*time.Millisecond, func(items []any) {
		received.Add(int64(len(items)))
	})
	defer b.Close()
	b.Submit(1, 1)
	time.Sleep(150 * time.Millisecond)
	if received.Load() != 1 {
		t.Fatal("应按时间触发")
	}
}

func TestBatch_Close(t *testing.T) {
	b := NewBatch(10, time.Second, nil)
	b.Submit("x", 1)
	b.Close()
	if b.Handled() != 1 {
		t.Fatal("Close 时应处理剩余")
	}
}
