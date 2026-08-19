// Package ringqueue 提供一个固定容量环形队列。
package ringqueue

import (
	"sync"
)

// Queue 是固定容量的环形队列。
type Queue struct {
	mu sync.Mutex
	d  []any
	hd int
	tl int
	ct int
}

// New 创建容量 cap 的环形队列。
func New(cap int) *Queue {
	if cap < 1 {
		cap = 16
	}
	return &Queue{d: make([]any, cap)}
}

// Push 把元素加入队尾。
func (q *Queue) Push(v any) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ct == len(q.d) {
		return false
	}
	q.d[q.tl] = v
	q.tl = (q.tl + 1) % len(q.d)
	q.ct++
	return true
}

// Pop 弹出队头元素。
func (q *Queue) Pop() (any, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ct == 0 {
		return nil, false
	}
	v := q.d[q.hd]
	q.d[q.hd] = nil
	q.hd = (q.hd + 1) % len(q.d)
	q.ct--
	return v, true
}

// Len 返回元素数。
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.ct
}

// Cap 返回容量。
func (q *Queue) Cap() int { return len(q.d) }

// Peek 返回队头但不弹出。
func (q *Queue) Peek() (any, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ct == 0 {
		return nil, false
	}
	return q.d[q.hd], true
}
