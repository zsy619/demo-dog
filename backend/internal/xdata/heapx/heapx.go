// Package heapx 提供最小堆 / 最大堆的辅助函数（基于 container/heap）。
package heapx

import "container/heap"

// MinHeap 是 int 最小堆。
type MinHeap []int

// Len 实现 sort.Interface。
func (h MinHeap) Len() int { return len(h) }

// Less 实现 sort.Interface。
func (h MinHeap) Less(i, j int) bool { return h[i] < h[j] }

// Swap 实现 sort.Interface。
func (h MinHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push 实现 heap.Interface。
func (h *MinHeap) Push(x any) { *h = append(*h, x.(int)) }

// Pop 实现 heap.Interface。
func (h *MinHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// MaxHeap 是 int 最大堆。
type MaxHeap []int

// Len 实现 sort.Interface。
func (h MaxHeap) Len() int { return len(h) }

// Less 实现 sort.Interface。
func (h MaxHeap) Less(i, j int) bool { return h[i] > h[j] }

// Swap 实现 sort.Interface。
func (h MaxHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push 实现 heap.Interface。
func (h *MaxHeap) Push(x any) { *h = append(*h, x.(int)) }

// Pop 实现 heap.Interface。
func (h *MaxHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// PushMin 把元素推入最小堆。
func PushMin(h *MinHeap, v int) { heap.Push(h, v) }

// PopMin 弹出最小元素。
func PopMin(h *MinHeap) int { return heap.Pop(h).(int) }

// PushMax 把元素推入最大堆。
func PushMax(h *MaxHeap, v int) { heap.Push(h, v) }

// PopMax 弹出最大元素。
func PopMax(h *MaxHeap) int { return heap.Pop(h).(int) }

// Heapify 对切片执行堆化（升序）。
func Heapify(s []int) {
	h := IntHeap(s)
	heap.Init(&h)
}

// IntHeap 是任意 cmp 的 int 堆。
type IntHeap []int

// Len 实现 sort.Interface。
func (h IntHeap) Len() int { return len(h) }

// Less 由外部 cmp 决定。
func (h IntHeap) Less(i, j int) bool { return h.cmp(i, j) }

// Swap 实现 sort.Interface。
func (h IntHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push 实现 heap.Interface。
func (h *IntHeap) Push(x any) { *h = append(*h, x.(int)) }

// Pop 实现 heap.Interface。
func (h *IntHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func (h IntHeap) cmp(i, j int) bool { return h[i] < h[j] }
