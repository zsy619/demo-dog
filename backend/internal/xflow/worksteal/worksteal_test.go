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

func TestPanic(t *testing.T) {
	s := New(2)
	s.Submit(func() { panic("boom") })
	s.Submit(func() {})
	s.Close()
	st := s.Stats()
	if st.Panics < 1 {
		t.Fatal("应记录 panic")
	}
}

func TestCloseIdempotent(t *testing.T) {
	s := New(2)
	s.Close()
	s.Close()
}

func TestSubmitAfterClose(t *testing.T) {
	s := New(1)
	s.Close()
	if s.Submit(func() {}) {
		t.Fatal("应拒绝")
	}
}

func TestNilJob(t *testing.T) {
	s := New(1)
	defer s.Close()
	if s.Submit(nil) {
		t.Fatal("nil job 应 false")
	}
}

func TestLoadDistribution(t *testing.T) {
	s := New(4)
	defer s.Close()
	for i := 0; i < 100; i++ {
		s.Submit(func() {})
	}
	s.Close()
	if s.Stats().Submits != 100 {
		t.Fatal("submit 计数")
	}
}
