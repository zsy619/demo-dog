package heapx

import "testing"

func TestMinPQ(t *testing.T) {
	pq := MinPQ[int]()
	for _, v := range []int{3, 1, 4, 1, 5, 9, 2, 6} {
		pq.Enqueue(v)
	}
	expected := []int{1, 1, 2, 3, 4, 5, 6, 9}
	for _, e := range expected {
		v, ok := pq.Dequeue()
		if !ok || v != e {
			t.Fatal("got", v, "want", e)
		}
	}
}

func TestMaxPQ(t *testing.T) {
	pq := MaxPQ[int]()
	for _, v := range []int{3, 1, 4, 1, 5, 9, 2, 6} {
		pq.Enqueue(v)
	}
	expected := []int{9, 6, 5, 4, 3, 2, 1, 1}
	for _, e := range expected {
		v, ok := pq.Dequeue()
		if !ok || v != e {
			t.Fatal("got", v, "want", e)
		}
	}
}

func TestCustomPriority(t *testing.T) {
	type task struct {
		name string
		pri  int
	}
	pq := New(func(a, b task) bool { return a.pri < b.pri }) // 小 pri 先出
	pq.Enqueue(task{"a", 3})
	pq.Enqueue(task{"b", 1})
	pq.Enqueue(task{"c", 2})
	a, _ := pq.Dequeue()
	if a.name != "b" {
		t.Fatal("first")
	}
}

func TestPeek(t *testing.T) {
	pq := MinPQ[int]()
	if _, ok := pq.Peek(); ok {
		t.Fatal("空 peek")
	}
	pq.Enqueue(5)
	pq.Enqueue(3)
	v, _ := pq.Peek()
	if v != 3 {
		t.Fatal("peek")
	}
}

func TestEmpty(t *testing.T) {
	pq := MinPQ[int]()
	if _, ok := pq.Dequeue(); ok {
		t.Fatal("应空")
	}
}

func TestItems(t *testing.T) {
	pq := MinPQ[int]()
	pq.Enqueue(2)
	pq.Enqueue(1)
	items := pq.Items()
	if len(items) != 2 {
		t.Fatal("items")
	}
}
