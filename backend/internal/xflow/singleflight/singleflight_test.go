package singleflight

import (
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
