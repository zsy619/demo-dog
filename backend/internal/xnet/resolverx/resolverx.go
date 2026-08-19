// Package resolverx 提供简单的 DNS 解析 + 缓存辅助。
package resolverx

import (
	"context"
	"net"
	"sync"
	"time"
)

// Resolver 是一个带 TTL 缓存的 DNS 解析器。
type Resolver struct {
	mu      sync.RWMutex
	cache   map[string]cacheEntry
	ttl     time.Duration
	resolve func(ctx context.Context, network, host string) ([]net.IP, error)
}

type cacheEntry struct {
	ips []net.IP
	at  time.Time
}

// New 创建一个使用 net.DefaultResolver 的解析器。
func New(ttl time.Duration) *Resolver {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Resolver{
		cache:   make(map[string]cacheEntry),
		ttl:     ttl,
		resolve: net.DefaultResolver.LookupIP,
	}
}

// LookupIP 解析 host，命中缓存则不发起查询。
func (r *Resolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	r.mu.RLock()
	e, ok := r.cache[host]
	r.mu.RUnlock()
	if ok && time.Since(e.at) < r.ttl {
		return e.ips, nil
	}
	ips, err := r.resolve(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[host] = cacheEntry{ips: ips, at: time.Now()}
	r.mu.Unlock()
	return ips, nil
}

// Invalidate 强制使某个 host 缓存失效。
func (r *Resolver) Invalidate(host string) {
	r.mu.Lock()
	delete(r.cache, host)
	r.mu.Unlock()
}

// Clear 清空缓存。
func (r *Resolver) Clear() {
	r.mu.Lock()
	r.cache = make(map[string]cacheEntry)
	r.mu.Unlock()
}

// Len 返回缓存条目数。
func (r *Resolver) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.cache)
}
