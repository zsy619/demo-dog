package executor

import (
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
