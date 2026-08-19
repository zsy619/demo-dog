package queuex

import "testing"

func TestPushPop(t *testing.T) {
	q := New[int](4)
	q.Push(1)
	q.Push(2)
	v, _ := q.Pop()
	if v != 1 {
		t.Fatal("pop", v)
	}
}

func TestOverflow(t *testing.T) {
	q := New[int](2)
	_, _ = q.Push(1)
	_, _ = q.Push(2)
	old, ok := q.Push(3)
	if !ok || old != 1 {
		t.Fatal("overflow", old)
	}
}

func TestLen(t *testing.T) {
	q := New[int](3)
	q.Push(1)
	q.Push(2)
	if q.Len() != 2 {
		t.Fatal("len", q.Len())
	}
}

func TestEmpty(t *testing.T) {
	q := New[int](4)
	if _, ok := q.Pop(); ok {
		t.Fatal("empty")
	}
}

func TestWrap(t *testing.T) {
	q := New[int](2)
	q.Push(1)
	q.Push(2)
	q.Pop()
	q.Push(3)
	v, _ := q.Pop()
	if v != 2 {
		t.Fatal("wrap", v)
	}
	v, _ = q.Pop()
	if v != 3 {
		t.Fatal("wrap2", v)
	}
}
