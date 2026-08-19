// Package heapx 包装 container/heap，提供可比较任意类型的优先队列。
package heapx

import "container/heap"

// Priority 是优先级的比较函数：返回 true 表示 a 优先级低于 b。
// 通常语义：低优先级先出（小顶堆）就是 a > b。
type Priority[T any] func(a, b T) bool

// PQ 是一个通用的优先队列。
type PQ[T any] struct {
	data []T
	less Priority[T]
}

// New 创建一个空优先队列。
func New[T any](less Priority[T]) *PQ[T] {
	return &PQ[T]{less: less}
}

// Len 返回元素数。
func (p *PQ[T]) Len() int { return len(p.data) }

// Push 压入元素（实现 heap.Interface）。
func (p *PQ[T]) Push(x any) { p.data = append(p.data, x.(T)) }

// Pop 弹出最小元素（实现 heap.Interface）。
func (p *PQ[T]) Pop() any {
	old := p.data
	n := len(old)
	x := old[n-1]
	p.data = old[:n-1]
	return x
}

// Less 比较两个元素（实现 heap.Interface）。
func (p *PQ[T]) Less(i, j int) bool { return p.less(p.data[i], p.data[j]) }

// Swap 交换（实现 heap.Interface）。
func (p *PQ[T]) Swap(i, j int) { p.data[i], p.data[j] = p.data[j], p.data[i] }

// Enqueue 入队一个元素。
func (p *PQ[T]) Enqueue(v T) {
	heap.Push(p, v)
}

// Dequeue 取出下一个元素。
func (p *PQ[T]) Dequeue() (T, bool) {
	var zero T
	if len(p.data) == 0 {
		return zero, false
	}
	return heap.Pop(p).(T), true
}

// Peek 查看顶部元素。
func (p *PQ[T]) Peek() (T, bool) {
	var zero T
	if len(p.data) == 0 {
		return zero, false
	}
	return p.data[0], true
}

// Items 返回底层快照。
func (p *PQ[T]) Items() []T {
	out := make([]T, len(p.data))
	copy(out, p.data)
	return out
}

// MinPQ 是小顶堆的便捷构造。
func MinPQ[T ~int | ~int32 | ~int64 | ~float32 | ~float64 | ~uint | ~uint32 | ~uint64 | ~string]() *PQ[T] {
	return New[T](func(a, b T) bool { return a < b })
}

// MaxPQ 是大顶堆的便捷构造。
func MaxPQ[T ~int | ~int32 | ~int64 | ~float32 | ~float64 | ~uint | ~uint32 | ~uint64 | ~string]() *PQ[T] {
	return New[T](func(a, b T) bool { return a > b })
}
