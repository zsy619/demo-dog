// Package queue 提供一个线程安全的 FIFO 队列。
package queue

import (
	"container/list"
	"sync"
)

// Queue 是 FIFO 队列。
type Queue[T any] struct {
	mu   sync.Mutex
	list *list.List
}

// New 创建一个空队列。
func New[T any]() *Queue[T] {
	return &Queue[T]{list: list.New()}
}

// Push 把元素入队。
func (q *Queue[T]) Push(v T) {
	q.mu.Lock()
	q.list.PushBack(v)
	q.mu.Unlock()
}

// Pop 弹出头元素；空时返回零值与 false。
func (q *Queue[T]) Pop() (T, bool) {
	var zero T
	q.mu.Lock()
	defer q.mu.Unlock()
	el := q.list.Front()
	if el == nil {
		return zero, false
	}
	q.list.Remove(el)
	return el.Value.(T), true
}

// Peek 查看头元素但不移除。
func (q *Queue[T]) Peek() (T, bool) {
	var zero T
	q.mu.Lock()
	defer q.mu.Unlock()
	el := q.list.Front()
	if el == nil {
		return zero, false
	}
	return el.Value.(T), true
}

// Len 返回元素数。
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.list.Len()
}

// Drain 清空并返回全部元素。
func (q *Queue[T]) Drain() []T {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]T, 0, q.list.Len())
	for e := q.list.Front(); e != nil; e = e.Next() {
		out = append(out, e.Value.(T))
	}
	q.list.Init()
	return out
}
