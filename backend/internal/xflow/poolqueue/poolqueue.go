// Package poolqueue 提供一个对象池化的队列（复用队列元素）。
package poolqueue

import "sync"

// Element 是池化队列元素。
type Element[T any] struct {
	Value T
	next  *Element[T]
}

// Queue 是一个对象池化链表队列。
// 通过把元素存在外部切片中并使用 *Element[T] 间接引用，
// 即使切片扩容也不会让已有指针失效。
type Queue[T any] struct {
	mu   sync.Mutex
	head *Element[T]
	tail *Element[T]
	pool []*Element[T]
	len  int
}

// New 创建一个空队列，poolSize 是预分配的元素池大小。
func New[T any](poolSize int) *Queue[T] {
	if poolSize < 0 {
		poolSize = 0
	}
	q := &Queue[T]{}
	for i := 0; i < poolSize; i++ {
		q.pool = append(q.pool, &Element[T]{})
	}
	return q
}

// Push 把元素加入队尾。
func (q *Queue[T]) Push(v T) {
	q.mu.Lock()
	el := q.alloc()
	el.Value = v
	el.next = nil
	if q.tail == nil {
		q.head = el
		q.tail = el
	} else {
		q.tail.next = el
		q.tail = el
	}
	q.len++
	q.mu.Unlock()
}

// Pop 弹出队头元素。
func (q *Queue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.head == nil {
		var zero T
		return zero, false
	}
	el := q.head
	q.head = el.next
	if q.head == nil {
		q.tail = nil
	}
	q.len--
	v := el.Value
	q.free(el)
	return v, true
}

// Len 返回元素数。
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.len
}

func (q *Queue[T]) alloc() *Element[T] {
	if n := len(q.pool); n > 0 {
		el := q.pool[n-1]
		q.pool = q.pool[:n-1]
		return el
	}
	return &Element[T]{}
}

func (q *Queue[T]) free(el *Element[T]) {
	var zero T
	el.Value = zero
	el.next = nil
	q.pool = append(q.pool, el)
}
