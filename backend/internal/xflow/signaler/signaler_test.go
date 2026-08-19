package signaler

import (
	"context"
	"testing"
)

func TestAcquireRelease(t *testing.T) {
	s := New(2)
	if err := s.Acquire(context.Background()); err != nil {
		t.Fatal("a1")
	}
	if err := s.Acquire(context.Background()); err != nil {
		t.Fatal("a2")
	}
	if s.TryAcquire() {
		t.Fatal("满")
	}
	s.Release()
	if !s.TryAcquire() {
		t.Fatal("应有空位")
	}
}

func TestGo(t *testing.T) {
	s := New(1)
	called := false
	err := s.Go(context.Background(), func() error {
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatal("go", err)
	}
}

func TestAvailable(t *testing.T) {
	s := New(3)
	if s.Available() != 3 {
		t.Fatal("init", s.Available())
	}
	s.Acquire(context.Background())
	if s.Available() != 2 {
		t.Fatal("after", s.Available())
	}
}

func TestCap(t *testing.T) {
	s := New(5)
	if s.Cap() != 5 {
		t.Fatal("cap")
	}
}
