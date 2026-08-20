// Package group 提供并发编排（errgroup 类似）：
// 限制并发数、任一失败可取消其余、收集错误。
//
// 用法：
//
//	g := group.New(ctx)
//	g.SetLimit(8)
//	for _, item := range items {
//		item := item
//		g.Go(func(ctx context.Context) error {
//			return process(ctx, item)
//		})
//	}
//	if err := g.Wait(); err != nil { return err }
//
// 行为：
//   - 任一任务返回 error：ctx 被取消，剩余任务应自行检查 ctx.Err()
//   - panic 在 fn 中被恢复并转为 error
//   - SetLimit 必须早于首个 Go
package group

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// Group 是并发任务编排器。
type Group struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	errs   []error
	limit  chan struct{}

	goCount    atomic.Int64
	doneCount  atomic.Int64
	errorCount atomic.Int64
}

// New 创建一个 Group；parent 为 nil 时使用 Background。
func New(parent context.Context) *Group {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Group{ctx: ctx, cancel: cancel}
}

// SetLimit 限制最大并发数（n <= 0 表示不限）。
// 必须在首个 Go 之前调用。
func (g *Group) SetLimit(n int) *Group {
	if n > 0 {
		g.limit = make(chan struct{}, n)
	}
	return g
}

// Go 启动一个 goroutine 执行 fn。
// fn 返回的 error 会导致 ctx 取消并被收集；panic 也会被恢复。
func (g *Group) Go(fn func(ctx context.Context) error) {
	if fn == nil {
		return
	}
	g.goCount.Add(1)
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		defer g.doneCount.Add(1)

		if g.limit != nil {
			select {
			case g.limit <- struct{}{}:
			case <-g.ctx.Done():
				// 已取消，不进入
				g.recordErr(g.ctx.Err())
				return
			}
			defer func() { <-g.limit }()
		}

		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("group: panic: %v", r)
				}
			}()
			err = fn(g.ctx)
		}()
		if err != nil {
			g.errorCount.Add(1)
			g.recordErr(err)
			g.cancel()
		}
	}()
}

func (g *Group) recordErr(err error) {
	g.mu.Lock()
	g.errs = append(g.errs, err)
	g.mu.Unlock()
}

// Wait 等待所有任务完成并返回错误（若有）。
// 多次调用安全，第二次返回首次结果。
func (g *Group) Wait() error {
	g.wg.Wait()
	// 多次调用安全：cancel 不重复调用
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
	return &MultiError{Errs: append([]error(nil), g.errs...)}
}

// Cancel 主动取消 ctx。
func (g *Group) Cancel() {
	if g.cancel != nil {
		g.cancel()
	}
}

// Stats 返回计数器快照。
type Stats struct {
	GoCount    int64 `json:"go_count"`
	DoneCount  int64 `json:"done_count"`
	ErrorCount int64 `json:"error_count"`
	ErrorList  int   `json:"error_list"`
}

// Stats 返回当前统计。
func (g *Group) Stats() Stats {
	g.mu.Lock()
	n := len(g.errs)
	g.mu.Unlock()
	return Stats{
		GoCount:    g.goCount.Load(),
		DoneCount:  g.doneCount.Load(),
		ErrorCount: g.errorCount.Load(),
		ErrorList:  n,
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
	parts := make([]string, len(m.Errs))
	for i, e := range m.Errs {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "; ")
}

// Unwrap 返回所有错误，使 errors.Is/As 能遍历整条错误链。
// 配合 errors.Join 语义：当任一子错误匹配 target 时返回 true。
func (m *MultiError) Unwrap() []error {
	if len(m.Errs) == 0 {
		return nil
	}
	out := make([]error, len(m.Errs))
	copy(out, m.Errs)
	return out
}

var _ error = (*MultiError)(nil)

// IsMulti 判断 err 是否是 MultiError。
var IsMulti = func(err error) bool {
	var m *MultiError
	return errors.As(err, &m)
}
