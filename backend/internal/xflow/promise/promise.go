// Package promise 提供一次性异步结果封装。
// 通过 Resolve / Reject 写入，Await 阻塞获取。
//
// 特性：
//   - AwaitCtx 支持 ctx 取消
//   - Run 中 fn panic 被恢复并转为 Reject
//   - All 不会泄漏未完成的 promise
package promise

import (
	"context"
	"fmt"
	"sync"
)

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

// Resolve 写入成功结果；幂等。
func (p *Promise[T]) Resolve(v T) {
	p.once.Do(func() {
		p.val = v
		close(p.done)
	})
}

// Reject 写入失败原因；幂等。
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

// AwaitCtx 阻塞直到有结果或 ctx 取消。
// ctx 取消时返回 ctx.Err()，但 promise 本身不会被自动取消。
func (p *Promise[T]) AwaitCtx(ctx context.Context) (T, error) {
	select {
	case <-p.done:
		return p.val, p.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
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
// fn panic 被恢复并转为 Reject。
func Run[T any](fn func() (T, error)) *Promise[T] {
	p := New[T]()
	go func() {
		v, err := func() (val T, e error) {
			defer func() {
				if r := recover(); r != nil {
					e = fmt.Errorf("promise: panic: %v", r)
				}
			}()
			return fn()
		}()
		if err != nil {
			p.Reject(err)
		} else {
			p.Resolve(v)
		}
	}()
	return p
}

// All 等待所有 Promise 完成；任一失败返回首个错误，\n// 但其他 promise 仍会被等待（不泄漏）。
func All[T any](ps ...*Promise[T]) ([]T, error) {
	out := make([]T, len(ps))
	var firstErr error
	for i, p := range ps {
		v, err := p.Await()
		out[i] = v
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return out, firstErr
}

// AllCtx 与 All 类似，但 ctx 取消时立即返回 ctx.Err()。
// 已被 Resolve 的 promise 仍会出现在返回值中。
func AllCtx[T any](ctx context.Context, ps ...*Promise[T]) ([]T, error) {
	out := make([]T, len(ps))
	var firstErr error
	for i, p := range ps {
		v, err := p.AwaitCtx(ctx)
		out[i] = v
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return out, firstErr
}
