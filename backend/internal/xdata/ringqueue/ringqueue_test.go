package ringqueue

import "testing"

func TestPushPop(t *testing.T) {
	q := New(2)
	if !q.Push(1) {
		t.Fatal("push")
	}
	if !q.Push(2) {
		t.Fatal("push2")
	}
	if q.Push(3) {
		t.Fatal("应满")
	}
	v, _ := q.Pop()
	if v.(int) != 1 {
		t.Fatal("pop")
	}
}

func TestPeek(t *testing.T) {
	q := New(4)
	q.Push("a")
	v, _ := q.Peek()
	if v.(string) != "a" {
		t.Fatal("peek")
	}
}

func TestLenCap(t *testing.T) {
	q := New(8)
	if q.Cap() != 8 {
		t.Fatal("cap")
	}
	q.Push(1)
	if q.Len() != 1 {
		t.Fatal("len")
	}
}

func TestEmpty(t *testing.T) {
	q := New(2)
	if _, ok := q.Pop(); ok {
		t.Fatal("empty")
	}
}
