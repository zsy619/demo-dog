package latch

import (
	"sync"
	"testing"
)

func TestRWLatch(t *testing.T) {
	l := NewRWLatch()
	l.RLock()
	l.RLock()
	if l.Stats().Readers != 2 {
		t.Fatal("readers")
	}
	l.RUnlock()
	l.RUnlock()
	l.Lock()
	if l.Stats().Writers != 1 {
		t.Fatal("writer")
	}
	l.Unlock()
}

func TestSimple(t *testing.T) {
	var s Simple
	s.Lock()
	s.Unlock()
}

func TestStriped_DifferentKeys(t *testing.T) {
	s := NewStriped(4)
	s.Lock("a")
	s.Lock("b")
	s.Unlock("a")
	s.Unlock("b")
}

func TestStriped_Do(t *testing.T) {
	s := NewStriped(8)
	called := false
	s.Do("k", func() { called = true })
	if !called {
		t.Fatal("Do")
	}
}

func TestStriped_Concurrent(t *testing.T) {
	s := NewStriped(16)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Do("k", func() {})
		}()
	}
	wg.Wait()
}

func TestRWLatch_Concurrent(t *testing.T) {
	l := NewRWLatch()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.RLock()
			l.RUnlock()
		}()
	}
	wg.Wait()
	if l.Stats().Readers != 0 {
		t.Fatal("计数")
	}
}
