// Package queuex 提供一个并发有界环形队列。
package queuex

import "sync"

// Queue 是一个有界 FIFO 队列。
type Queue[T any] struct {
	mu   sync.Mutex
	cap  int
	buf  []T
	head int
	tail int
	full bool
}

// New 创建容量 cap 的有界队列。
func New[T any](cap int) *Queue[T] {
	if cap < 1 {
		cap = 1
	}
	return &Queue[T]{cap: cap, buf: make([]T, cap)}
}

// Push 推入队尾，满则覆盖最旧元素并返回被覆盖的旧值与 true。
// 返回 (zero, false) 表示未覆盖。
func (q *Queue[T]) Push(v T) (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var zero T
	if q.full {
		old := q.buf[q.head]
		q.head = (q.head + 1) % q.cap
		q.full = false
		q.buf[q.tail] = v
		q.tail = (q.tail + 1) % q.cap
		if q.head == q.tail {
			q.full = true
		}
		return old, true
	}
	q.buf[q.tail] = v
	q.tail = (q.tail + 1) % q.cap
	if q.head == q.tail {
		q.full = true
	}
	return zero, false
}

// Pop 弹出队头。
func (q *Queue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var zero T
	if q.head == q.tail && !q.full {
		return zero, false
	}
	v := q.buf[q.head]
	q.head = (q.head + 1) % q.cap
	q.full = false
	return v, true
}

// Len 返回当前元素数。
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	switch {
	case q.full:
		return q.cap
	case q.tail >= q.head:
		return q.tail - q.head
	default:
		return q.cap - q.head + q.tail
	}
}

// Cap 返回容量。
func (q *Queue[T]) Cap() int { return q.cap }
