// Package rwqueue 提供基于 chan 的多生产者多消费者队列。
package rwqueue

import (
	"context"
	"sync"
)

// Queue 是一个带缓冲的 FIFO 队列，提供阻塞/非阻塞入队/出队。
type Queue[T any] struct {
	ch   chan T
	stat sync.RWMutex
	cap  int
}

// New 创建一个容量 cap 的队列。
func New[T any](cap int) *Queue[T] {
	if cap < 1 {
		cap = 16
	}
	return &Queue[T]{ch: make(chan T, cap), cap: cap}
}

// Push 阻塞入队。
func (q *Queue[T]) Push(v T) {
	q.ch <- v
}

// TryPush 非阻塞入队。
func (q *Queue[T]) TryPush(v T) bool {
	select {
	case q.ch <- v:
		return true
	default:
		return false
	}
}

// PushCtx 阻塞入队，ctx 取消时返回错误。
func (q *Queue[T]) PushCtx(ctx context.Context, v T) error {
	select {
	case q.ch <- v:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Pop 阻塞出队。
func (q *Queue[T]) Pop() (T, bool) {
	v, ok := <-q.ch
	return v, ok
}

// PopCtx 阻塞出队，ctx 取消时返回零值与错误。
func (q *Queue[T]) PopCtx(ctx context.Context) (T, error) {
	var zero T
	select {
	case v, ok := <-q.ch:
		if !ok {
			return zero, ErrClosed
		}
		return v, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// TryPop 非阻塞出队。
func (q *Queue[T]) TryPop() (T, bool) {
	var zero T
	select {
	case v, ok := <-q.ch:
		return v, ok
	default:
		return zero, false
	}
}

// Len 返回队列长度。
func (q *Queue[T]) Len() int { return len(q.ch) }

// Cap 返回队列容量。
func (q *Queue[T]) Cap() int { return q.cap }

// Close 关闭队列。
func (q *Queue[T]) Close() {
	q.stat.Lock()
	defer q.stat.Unlock()
	close(q.ch)
}

// ErrClosed 表示队列已关闭。
var ErrClosed = errClosed{}

type errClosed struct{}

func (errClosed) Error() string { return "rwqueue: 已关闭" }
