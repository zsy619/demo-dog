package scheduler

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	s := New()
	if err := s.Add(Job{Name: "x", Interval: time.Millisecond, Fn: func() {}}); err != nil {
		t.Fatal(err)
	}
}

func TestAdd_Nil(t *testing.T) {
	s := New()
	if err := s.Add(Job{Name: "x"}); !errors.Is(err, ErrNilFn) {
		t.Fatal("应 ErrNilFn")
	}
}

func TestRun(t *testing.T) {
	var n atomic.Int64
	s := New()
	s.Add(Job{Name: "a", Interval: 20 * time.Millisecond, Fn: func() { n.Add(1) }})
	s.Start()
	time.Sleep(150 * time.Millisecond)
	s.Stop()
	if n.Load() < 3 {
		t.Fatal("应至少触发 3 次")
	}
}

func TestPanicRecovers(t *testing.T) {
	s := New()
	s.Add(Job{Name: "p", Interval: 20 * time.Millisecond, Fn: func() {
		panic("oops")
	}})
	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()
}

func TestDoubleStart(t *testing.T) {
	s := New()
	s.Add(Job{Name: "x", Interval: 100 * time.Millisecond, Fn: func() {}})
	s.Start()
	s.Start() // 不应启动两次
	s.Stop()
}
