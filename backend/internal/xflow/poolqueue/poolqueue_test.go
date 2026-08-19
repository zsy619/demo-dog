package poolqueue

import "testing"

func TestPushPop(t *testing.T) {
	q := New[int](4)
	q.Push(1)
	q.Push(2)
	v, _ := q.Pop()
	if v != 1 {
		t.Fatal("pop", v)
	}
	v, _ = q.Pop()
	if v != 2 {
		t.Fatal("pop2", v)
	}
}

func TestEmpty(t *testing.T) {
	q := New[int](2)
	if _, ok := q.Pop(); ok {
		t.Fatal("empty")
	}
}

func TestLen(t *testing.T) {
	q := New[int](4)
	q.Push(1)
	q.Push(2)
	if q.Len() != 2 {
		t.Fatal("len")
	}
	q.Pop()
	if q.Len() != 1 {
		t.Fatal("len after pop")
	}
}

func TestPoolReuse(t *testing.T) {
	q := New[int](4)
	for i := 0; i < 100; i++ {
		q.Push(i)
		q.Pop()
	}
	if q.Len() != 0 {
		t.Fatal("reuse", q.Len())
	}
}
