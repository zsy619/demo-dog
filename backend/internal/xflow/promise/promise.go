// Package promise 提供一次性异步结果封装。
// 通过 Resolve / Reject 写入，Await 阻塞获取。
package promise

import "sync"

// Promise 是一个一次性异步结果。
type Promise[T any] struct {
	once sync.Once
	val  T
	err  error
	done chan struct{}
}

// New 创建一个未完成的 Promise。
func New[T any]() *Promise[T] {
	return &Promise[T]{done: make(chan struct{})}
}

// Resolve 写入成功结果。
func (p *Promise[T]) Resolve(v T) {
	p.once.Do(func() {
		p.val = v
		close(p.done)
	})
}

// Reject 写入失败原因。
func (p *Promise[T]) Reject(err error) {
	p.once.Do(func() {
		p.err = err
		close(p.done)
	})
}

// Await 阻塞直到有结果。
func (p *Promise[T]) Await() (T, error) {
	<-p.done
	return p.val, p.err
}

// IsDone 返回是否已经写入。
func (p *Promise[T]) IsDone() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// Run 在 goroutine 中执行 fn 并把结果写入新 Promise。
func Run[T any](fn func() (T, error)) *Promise[T] {
	p := New[T]()
	go func() {
		v, err := fn()
		if err != nil {
			p.Reject(err)
		} else {
			p.Resolve(v)
		}
	}()
	return p
}

// All 等待所有 Promise 完成。
func All[T any](ps ...*Promise[T]) ([]T, error) {
	out := make([]T, len(ps))
	for i, p := range ps {
		v, err := p.Await()
		out[i] = v
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
