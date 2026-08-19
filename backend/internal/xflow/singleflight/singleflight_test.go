package singleflight

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDo(t *testing.T) {
	g := New[int]()
	v, err := g.Do("k", func() (int, error) { return 1, nil })
	if err != nil || v != 1 {
		t.Fatal("do")
	}
}

func TestMerge(t *testing.T) {
	g := New[int]()
	var n atomic.Int32
	start := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(start)
		v, _ := g.Do("k", func() (int, error) {
			n.Add(1)
			<-time.After(50 * time.Millisecond)
			return 42, nil
		})
		if v != 42 {
			t.Fatal("v", v)
		}
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
	if n.Load() != 1 {
		t.Fatal("merge missed", n.Load())
	}
}
