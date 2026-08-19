// Package breaker 提供并发上限断路器：
// 当活跃请求数达到 limit 时，新调用立即失败并返回 ErrOverload。
package breaker

import (
	"context"
	"errors"
	"sync/atomic"
)

// ErrOverload 在并发超出上限时返回。
var ErrOverload = errors.New("breaker: 并发超限")

// Breaker 是一个并发上限控制器。
type Breaker struct {
	limit  int
	active atomic.Int32
	wait   atomic.Int32
	total  atomic.Uint64
	fails  atomic.Uint64
}

// New 创建一个 limit 上限的断路器。
func New(limit int) *Breaker {
	if limit <= 0 {
		limit = 100
	}
	return &Breaker{limit: limit}
}

// Acquire 占用一个许可。非阻塞。
func (b *Breaker) Acquire() (release func(), ok bool) {
	b.total.Add(1)
	if b.active.Add(1) > int32(b.limit) {
		b.active.Add(-1)
		b.fails.Add(1)
		return func() {}, false
	}
	return func() { b.active.Add(-1) }, true
}

// Run 执行 fn；超限直接返回 ErrOverload。
func (b *Breaker) Run(fn func() error) error {
	release, ok := b.Acquire()
	defer release()
	if !ok {
		return ErrOverload
	}
	return fn()
}

// RunCtx 带 context 的执行。
func (b *Breaker) RunCtx(ctx context.Context, fn func(context.Context) error) error {
	release, ok := b.Acquire()
	defer release()
	if !ok {
		return ErrOverload
	}
	return fn(ctx)
}

// Active 返回当前活跃数。
func (b *Breaker) Active() int { return int(b.active.Load()) }

// Stats 返回计数视图。
type Stats struct {
	Active int    `json:"active"`
	Limit  int    `json:"limit"`
	Total  uint64 `json:"total"`
	Fails  uint64 `json:"fails"`
}

// Stats 返回当前统计。
func (b *Breaker) Stats() Stats {
	return Stats{Active: b.Active(), Limit: b.limit, Total: b.total.Load(), Fails: b.fails.Load()}
}

// SetLimit 动态修改上限。
func (b *Breaker) SetLimit(n int) {
	if n <= 0 {
		return
	}
	b.limit = n
}
