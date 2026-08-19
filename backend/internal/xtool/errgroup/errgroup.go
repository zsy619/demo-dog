// Package errgroup 提供一组并行任务，任一失败则全部停止等待。
package errgroup

import (
	"context"
	"sync"
)

// Group 是一个错误组。
type Group struct {
	wg  sync.WaitGroup
	ctx context.Context

	cancel     context.CancelFunc
	firstErr   error
	firstErrMu sync.Mutex
}

// New 创建一个 Group。
func New() *Group { return &Group{} }

// WithContext 返回带 ctx 的 Group。
func WithContext(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &Group{ctx: ctx, cancel: cancel}, ctx
}

// Go 启动一个任务；返回 false 表示已失败。
func (g *Group) Go(f func() error) {
	if g.cancel != nil {
		g.wg.Add(1)
		go func() {
			defer g.wg.Done()
			if err := f(); err != nil {
				g.setErr(err)
				if g.cancel != nil {
					g.cancel()
				}
			}
		}()
		return
	}
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := f(); err != nil {
			g.setErr(err)
		}
	}()
}

// Wait 等待全部完成，返回首个错误。
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.firstErr
}

func (g *Group) setErr(err error) {
	g.firstErrMu.Lock()
	if g.firstErr == nil {
		g.firstErr = err
	}
	g.firstErrMu.Unlock()
}
