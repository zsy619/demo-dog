package queue

import "testing"

func TestPushPop(t *testing.T) {
	q := New[int]()
	q.Push(1)
	q.Push(2)
	if v, _ := q.Pop(); v != 1 {
		t.Fatal("pop 1")
	}
	if v, _ := q.Pop(); v != 2 {
		t.Fatal("pop 2")
	}
}

func TestPop_Empty(t *testing.T) {
	q := New[int]()
	if _, ok := q.Pop(); ok {
		t.Fatal("应空")
	}
}

func TestPeek(t *testing.T) {
	q := New[int]()
	q.Push(1)
	v, _ := q.Peek()
	if v != 1 {
		t.Fatal("peek")
	}
	if q.Len() != 1 {
		t.Fatal("peek 不应消耗")
	}
}

func TestLen(t *testing.T) {
	q := New[int]()
	if q.Len() != 0 {
		t.Fatal("empty")
	}
	q.Push(1)
	if q.Len() != 1 {
		t.Fatal("len")
	}
}

func TestDrain(t *testing.T) {
	q := New[int]()
	q.Push(1)
	q.Push(2)
	q.Push(3)
	d := q.Drain()
	if len(d) != 3 {
		t.Fatal("drain")
	}
	if q.Len() != 0 {
		t.Fatal("应清空")
	}
}
