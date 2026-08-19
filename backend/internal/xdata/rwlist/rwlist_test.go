package rwlist

import "testing"

func TestPushPop(t *testing.T) {
	l := New[int]()
	l.PushBack(1)
	l.PushBack(2)
	v, _ := l.PopFront()
	if v != 1 {
		t.Fatal("front", v)
	}
	v, _ = l.PopBack()
	if v != 2 {
		t.Fatal("back", v)
	}
}

func TestEmpty(t *testing.T) {
	l := New[int]()
	if _, ok := l.PopFront(); ok {
		t.Fatal("empty")
	}
}

func TestRange(t *testing.T) {
	l := New[int]()
	l.PushBack(1)
	l.PushBack(2)
	l.PushBack(3)
	sum := 0
	l.Range(func(v int) bool { sum += v; return true })
	if sum != 6 {
		t.Fatal("range", sum)
	}
}

func TestClear(t *testing.T) {
	l := New[int]()
	l.PushBack(1)
	l.Clear()
	if l.Len() != 0 {
		t.Fatal("clear")
	}
}
