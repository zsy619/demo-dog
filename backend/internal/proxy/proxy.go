package proxy

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Backend is a single upstream.
type Backend struct {
	Name   string
	URL    string
	Alive  atomic.Bool
	Weight int
}

// BackendPicker picks a backend per call. Strategy is the
// caller-side: the Pool returns a stable pick by index.
type Pool struct {
	mu       sync.RWMutex
	backends []*Backend
	cursor   int
	picks    atomic.Uint64
	misses   atomic.Uint64
}

// New creates a Pool with the given backends.
func New(backends []*Backend) *Pool {
	for _, b := range backends {
		b.Alive.Store(true)
	}
	return &Pool{backends: backends}
}

// ErrNoBackend is returned when no backend is alive.
var ErrNoBackend = errors.New("no backend available")

// Pick returns the next live backend via round-robin.
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

// MarkUp / MarkDown toggle a backend's alive state.
func (p *Pool) MarkUp(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range p.backends {
		if b.Name == name {
			b.Alive.Store(true)
		}
	}
}

// MarkDown sets a backend dead.
func (p *Pool) MarkDown(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range p.backends {
		if b.Name == name {
			b.Alive.Store(false)
		}
	}
}

// Backends returns a snapshot of backends with their alive
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

// BackendView is a snapshot of a backend's state.
type BackendView struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Alive  bool   `json:"alive"`
	Weight int    `json:"weight"`
}

// Stats returns the counter snapshot.
type Stats struct {
	Picks   uint64       `json:"picks"`
	Misses  uint64       `json:"misses"`
	Backends []BackendView `json:"backends"`
}

// Stats returns the snapshot.
func (p *Pool) Stats() Stats {
	return Stats{Picks: p.picks.Load(), Misses: p.misses.Load(), Backends: p.Backends()}
}

// BackendCall is a single call to a backend.
type BackendCall func(ctx context.Context, b *Backend) error

// DoWithFallback iterates backends in order until one succeeds.
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
