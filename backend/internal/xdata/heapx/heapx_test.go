package heapx

import (
	"container/heap"
	"testing"
)

func TestMinHeap(t *testing.T) {
	h := &MinHeap{3, 1, 4, 1, 5}
	containerheapInit(*h)
	PushMin(h, 9)
	if PopMin(h) != 1 {
		t.Fatal("min")
	}
}

func TestMaxHeap(t *testing.T) {
	h := &MaxHeap{3, 1, 4, 1, 5}
	PushMax(h, 9)
	if PopMax(h) != 9 {
		t.Fatal("max")
	}
}

func TestHeapify(t *testing.T) {
	s := []int{3, 1, 4, 1, 5}
	Heapify(s)
	if s[0] != 1 {
		t.Fatal("heapify", s)
	}
}

func containerheapInit(s MinHeap) { heap.Init(&s) }

func containerheapInit2(h *IntHeap) { heap.Init(h) }

func TestIntHeap(t *testing.T) {
	h := IntHeap{2, 8, 5}
	containerheapInit2(&h)
	if h[0] != 2 {
		t.Fatal("ih")
	}
}
