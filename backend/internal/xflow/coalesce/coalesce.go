// Package coalesce 提供按 key 合并多个并发调用的工具：
// 同一 key 的并发请求只执行一次回调，其余等待者共享结果。
package coalesce

import (
	"sync"
	"sync/atomic"
)

// Result 是合并调用的结果。
type Result struct {
	Value any
	Err   error
}

// Flight 是单 key 的飞行中调用记录。
type Flight struct {
	done  chan struct{}
	res   Result
	mu    sync.Mutex
	wait  atomic.Int32
}

// Group 持有多个 key 的 Flight。
type Group struct {
	mu       sync.Mutex
	table    map[string]*Flight
}

// NewGroup 创建一个空 Group。
func NewGroup() *Group {
	return &Group{table: make(map[string]*Flight)}
}

// Do 在 key 上执行 fn；并发请求共享同一结果。
func (g *Group) Do(key string, fn func() (any, error)) (any, error, bool) {
	g.mu.Lock()
	if f, ok := g.table[key]; ok {
		f.wait.Add(1)
		g.mu.Unlock()
		<-f.done
		return f.res.Value, f.res.Err, false
	}
	f := &Flight{done: make(chan struct{}), wait: atomic.Int32{}}
	f.wait.Add(1)
	g.table[key] = f
	g.mu.Unlock()
	v, err := fn()
	f.res = Result{Value: v, Err: err}
	close(f.done)
	g.mu.Lock()
	delete(g.table, key)
	g.mu.Unlock()
	return v, err, true
}

// Len 返回当前飞行中的 key 数。
func (g *Group) Len() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.table)
}

// Forget 主动取消一个 key 的合并（后续 Do 重新执行）。
func (g *Group) Forget(key string) {
	g.mu.Lock()
	delete(g.table, key)
	g.mu.Unlock()
}
