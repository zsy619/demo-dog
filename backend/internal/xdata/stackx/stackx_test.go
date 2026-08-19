package stackx

import "testing"

func TestPushPop(t *testing.T) {
	s := New[int]()
	s.Push(1)
	s.Push(2)
	if v, _ := s.Pop(); v != 2 {
		t.Fatal("pop")
	}
}

func TestPop_Empty(t *testing.T) {
	s := New[int]()
	if _, ok := s.Pop(); ok {
		t.Fatal("empty")
	}
}

func TestPeek(t *testing.T) {
	s := New[int]()
	s.Push(1)
	v, _ := s.Peek()
	if v != 1 {
		t.Fatal("peek")
	}
	if s.Len() != 1 {
		t.Fatal("len")
	}
}

func TestClear(t *testing.T) {
	s := New[int]()
	s.Push(1)
	s.Clear()
	if s.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestLen(t *testing.T) {
	s := New[int]()
	if s.Len() != 0 {
		t.Fatal("empty")
	}
	s.Push(1)
	if s.Len() != 1 {
		t.Fatal("len")
	}
}
