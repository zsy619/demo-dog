// Package singleflight 提供泛型 SingleFlight 语义。
// 同一 key 并发调用会合并为一次实际执行；返回值与 key 均为类型参数。
//
// 特性：
//   - panic 在 fn 中被恢复，转为 error 返回给所有等待者
//   - DoCtx 支持 ctx 取消（取消时立即返回，不等待 in-flight call）
//   - Forget 强制重新执行
//   - Inflight 监控
package singleflight

import (
	"context"
	"fmt"
	"sync"
)

// Group 是合并相同 key 调用的容器。
type Group[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]*call[V]
}

type call[V any] struct {
	wg  sync.WaitGroup
	val V
	err error
}

// New 创建一个 Group。
func New[K comparable, V any]() *Group[K, V] {
	return &Group[K, V]{m: make(map[K]*call[V])}
}

// Do 在 fn 中执行针对 key 的实际操作。
// fn 的 panic 会被恢复并转为 error 返回给所有等待者。
func (g *Group[K, V]) Do(key K, fn func() (V, error)) (V, error) {
	return g.doCtx(context.Background(), key, fn)
}

// DoCtx 与 Do 类似，但若 ctx 取消则立即返回 ctx.Err()。
// 注意：取消仅影响当前调用者；in-flight call 不会被中断。
func (g *Group[K, V]) DoCtx(ctx context.Context, key K, fn func() (V, error)) (V, error) {
	if err := ctx.Err(); err != nil {
		var zero V
		return zero, err
	}
	return g.doCtx(ctx, key, fn)
}

func (g *Group[K, V]) doCtx(ctx context.Context, key K, fn func() (V, error)) (V, error) {
	g.mu.Lock()
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		// 等待 in-flight 完成或 ctx 取消
		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			return c.val, c.err
		case <-ctx.Done():
			var zero V
			return zero, ctx.Err()
		}
	}
	c := &call[V]{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	// 执行 fn，捕获 panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				c.err = fmt.Errorf("singleflight: panic: %v", r)
			}
			c.wg.Done()
			g.mu.Lock()
			delete(g.m, key)
			g.mu.Unlock()
		}()
		c.val, c.err = fn()
	}()

	if err := ctx.Err(); err != nil {
		var zero V
		return zero, err
	}
	return c.val, c.err
}

// Forget 删除一个正在执行的 key（强制后续调用重新执行）。
// 用于外部取消场景。
func (g *Group[K, V]) Forget(key K) {
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
}

// Inflight 返回正在执行的 key 数。
func (g *Group[K, V]) Inflight() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.m)
}
