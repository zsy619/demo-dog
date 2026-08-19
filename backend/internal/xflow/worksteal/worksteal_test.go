package worksteal

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestSubmit(t *testing.T) {
	s := New(2)
	defer s.Close()
	var n atomic.Int32
	for i := 0; i < 50; i++ {
		s.Submit(func() {
			n.Add(1)
		})
	}
	time.Sleep(100 * time.Millisecond)
	if n.Load() != 50 {
		t.Fatal("missed", n.Load())
	}
}

func TestClose(t *testing.T) {
	s := New(2)
	s.Submit(func() {})
	s.Close()
	// 再 Close 应不 panic
	s.Close()
}
