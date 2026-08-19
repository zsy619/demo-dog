package seq

import "testing"

func TestNext(t *testing.T) {
	s := New(0)
	if s.Next() != 0 {
		t.Fatal("first")
	}
	if s.Next() != 1 {
		t.Fatal("second")
	}
}

func TestPeek(t *testing.T) {
	s := New(10)
	if s.Peek() != 10 {
		t.Fatal("peek")
	}
	s.Next()
	if s.Peek() != 11 {
		t.Fatal("after")
	}
}

func TestReset(t *testing.T) {
	s := New(0)
	s.Next()
	s.Reset(100)
	if s.Peek() != 100 {
		t.Fatal("reset")
	}
}

func TestSetCAS(t *testing.T) {
	s := New(0)
	if !s.SetCAS(0, 5) {
		t.Fatal("cas")
	}
	if s.Peek() != 5 {
		t.Fatal("after")
	}
	if s.SetCAS(0, 7) {
		t.Fatal("应失败")
	}
}

func TestStart(t *testing.T) {
	s := New(100)
	if s.Next() != 100 {
		t.Fatal("start")
	}
}
