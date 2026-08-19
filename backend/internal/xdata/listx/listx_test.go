package listx

import "testing"

func TestPush(t *testing.T) {
	l := New[int]()
	l.PushBack(1)
	l.PushBack(2)
	if l.Len() != 2 {
		t.Fatal("len")
	}
}

func TestFront(t *testing.T) {
	l := New[int]()
	l.PushBack(1)
	l.PushBack(2)
	if v := l.Front().Value(); v != 1 {
		t.Fatal("front", v)
	}
}

func TestBack(t *testing.T) {
	l := New[int]()
	l.PushBack(1)
	l.PushBack(2)
	if v := l.Back().Value(); v != 2 {
		t.Fatal("back")
	}
}

func TestRange(t *testing.T) {
	l := New[int]()
	l.PushBack(1)
	l.PushBack(2)
	sum := 0
	l.Range(func(v int) bool { sum += v; return true })
	if sum != 3 {
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

func TestNext(t *testing.T) {
	l := New[int]()
	l.PushBack(1)
	l.PushBack(2)
	n := l.Front().Next()
	if n == nil || n.Value() != 2 {
		t.Fatal("next")
	}
}
