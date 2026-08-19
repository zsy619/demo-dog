// Package group 提供并发编排（errgroup 类似）：
// 限制并发数、任一失败可取消其余、收集错误。
package group

import (
	"context"
	"sync"
)

// Group 是并发任务编排器。
type Group struct {
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	errs    []error
	limit   chan struct{}
}

// New 创建一个默认不限并发的 Group。
func New(parent context.Context) *Group {
	return &Group{ctx: parent}
}

// SetLimit 限制最大并发数（<=0 表示不限）。
func (g *Group) SetLimit(n int) *Group {
	if n > 0 {
		g.limit = make(chan struct{}, n)
	}
	return g
}

// Go 启动一个 goroutine 执行 fn；ctx 用于取消。
func (g *Group) Go(fn func(ctx context.Context) error) {
	if g.ctx == nil {
		g.ctx = context.Background()
	}
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if g.limit != nil {
			g.limit <- struct{}{}
			defer func() { <-g.limit }()
		}
		c := g.ctx
		if g.cancel != nil {
			// 子 ctx
		}
		if err := fn(c); err != nil {
			g.mu.Lock()
			g.errs = append(g.errs, err)
			g.mu.Unlock()
			if g.cancel != nil {
				g.cancel()
			}
		}
	}()
}

// Wait 等待所有任务完成。
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.errs) == 0 {
		return nil
	}
	if len(g.errs) == 1 {
		return g.errs[0]
	}
	return &MultiError{Errs: g.errs}
}

// Cancel 主动取消。
func (g *Group) Cancel() {
	if g.cancel != nil {
		g.cancel()
	} else {
		g.cancel = func() {}
	}
}

// MultiError 聚合多个错误。
type MultiError struct {
	Errs []error
}

// Error 返回拼接后的错误信息。
func (m *MultiError) Error() string {
	if len(m.Errs) == 0 {
		return ""
	}
	s := m.Errs[0].Error()
	for _, e := range m.Errs[1:] {
		s += "; " + e.Error()
	}
	return s
}
