// Package proxy HTTP/SOCKS 代理：URL 解析与连接转发。
package proxy

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Backend 是一个上游。
type Backend struct {
	Name   string
	URL    string
	Alive  atomic.Bool
	Weight int
}

// BackendPicker 每次调用选取一个 backend。策略由调用方侧决定：
// Pool 按索引返回一个稳定的 pick。
type Pool struct {
	mu       sync.RWMutex
	backends []*Backend
	cursor   int
	picks    atomic.Uint64
	misses   atomic.Uint64
}

// New 用给定的 backends 创建一个 Pool。
func New(backends []*Backend) *Pool {
	for _, b := range backends {
		b.Alive.Store(true)
	}
	return &Pool{backends: backends}
}

// ErrNoBackend 在没有存活 backend 时返回。
var ErrNoBackend = errors.New("no backend available")

// Pick 通过轮询返回下一个存活的 backend。
func (p *Pool) Pick() (*Backend, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.backends) == 0 {
		return nil, ErrNoBackend
	}
	for i := 0; i < len(p.backends); i++ {
		idx := (p.cursor + i) % len(p.backends)
		b := p.backends[idx]
		if b.Alive.Load() {
			p.cursor = (idx + 1) % len(p.backends)
			p.picks.Add(1)
			return b, nil
		}
	}
	p.misses.Add(1)
	return nil, ErrNoBackend
}

// MarkUp / MarkDown 切换 backend 的存活状态。
func (p *Pool) MarkUp(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range p.backends {
		if b.Name == name {
			b.Alive.Store(true)
		}
	}
}

// MarkDown 将一个 backend 标记为下线。
func (p *Pool) MarkDown(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range p.backends {
		if b.Name == name {
			b.Alive.Store(false)
		}
	}
}

// Backends 返回 a 快照 of backends with their alive
// state.
func (p *Pool) Backends() []BackendView {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]BackendView, 0, len(p.backends))
	for _, b := range p.backends {
		out = append(out, BackendView{Name: b.Name, URL: b.URL, Alive: b.Alive.Load(), Weight: b.Weight})
	}
	return out
}

// BackendView 是 backend 状态的一个快照。
type BackendView struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Alive  bool   `json:"alive"`
	Weight int    `json:"weight"`
}

// Stats 返回计数器的快照。
type Stats struct {
	Picks   uint64       `json:"picks"`
	Misses  uint64       `json:"misses"`
	Backends []BackendView `json:"backends"`
}

// Stats 返回快照。
func (p *Pool) Stats() Stats {
	return Stats{Picks: p.picks.Load(), Misses: p.misses.Load(), Backends: p.Backends()}
}

// BackendCall 是对 backend 的一次调用。
type BackendCall func(ctx context.Context, b *Backend) error

// DoWithFallback 按顺序遍历 backend，直到有一个成功。
func (p *Pool) DoWithFallback(ctx context.Context, call BackendCall) error {
	p.mu.Lock()
	bs := make([]*Backend, len(p.backends))
	copy(bs, p.backends)
	p.mu.Unlock()
	var last error
	for _, b := range bs {
		if !b.Alive.Load() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := call(ctx, b); err != nil {
			last = err
			continue
		}
		return nil
	}
	if last == nil {
		return ErrNoBackend
	}
	return last
}
