package spinlock

import "testing"

func TestTryLock(t *testing.T) {
	var s SpinLock
	if !s.TryLock() {
		t.Fatal("try")
	}
	if s.TryLock() {
		t.Fatal("应失败")
	}
	s.Unlock()
	if !s.TryLock() {
		t.Fatal("try 2")
	}
	s.Unlock()
}

func TestDo(t *testing.T) {
	var s SpinLock
	called := false
	s.Do(func() { called = true })
	if !called {
		t.Fatal("do")
	}
}

func TestLock(t *testing.T) {
	var s SpinLock
	s.Lock()
	s.Unlock()
	s.Lock()
	s.Unlock()
}
