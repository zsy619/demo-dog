// Package singleflight 提供按 key 合并重复并发调用的同步原语：
// 同一 key 上正在执行的请求会被后续请求共享，避免重复计算或网络调用。
package singleflight

import (
	"sync"
)

// call 代表一个飞行中的请求。
type call struct {
	wg  sync.WaitGroup
	val any
	err error
}

// Group 是单飞行协调器。
type Group struct {
	mu    sync.Mutex
	calls map[string]*call
}

// New 创建一个空 Group。
func New() *Group {
	return &Group{calls: make(map[string]*call)}
}

// Do 在 key 上执行 fn；如果已有飞行中调用则等待其结果。
// shared 返回 true 表示结果被多个调用方共享。
func (g *Group) Do(key string, fn func() (any, error)) (v any, err error, shared bool) {
	g.mu.Lock()
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}
	c := new(call)
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()
	c.val, c.err = fn()
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	c.wg.Done()
	return c.val, c.err, false
}

// Forget 主动取消 key 上的合并（若有进行中的请求，等待其自然完成）。
func (g *Group) Forget(key string) {
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
}

// InFlight 返回当前飞行中的 key 数。
func (g *Group) InFlight() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.calls)
}
