// Package resolverx 提供简单的 DNS 解析 + 缓存辅助。
// 使用 singleflight 防止同一 host 的并发请求触发重复 DNS 查询。
package resolverx

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Resolver 是一个带 TTL 缓存的 DNS 解析器。
type Resolver struct {
	mu      sync.Mutex
	cache   map[string]cacheEntry
	flights map[string]*flight
	ttl     time.Duration
	resolve func(ctx context.Context, network, host string) ([]net.IP, error)
}

type cacheEntry struct {
	ips []net.IP
	at  time.Time
}

type flight struct {
	done chan struct{}
	ips  []net.IP
	err  error
}

// New 创建一个使用 net.DefaultResolver 的解析器。
func New(ttl time.Duration) *Resolver {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Resolver{
		cache:   make(map[string]cacheEntry),
		flights: make(map[string]*flight),
		ttl:     ttl,
		resolve: net.DefaultResolver.LookupIP,
	}
}

// LookupIP 解析 host，命中缓存则不发起查询。
func (r *Resolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	r.mu.Lock()
	if e, ok := r.cache[host]; ok && time.Since(e.at) < r.ttl {
		r.mu.Unlock()
		return e.ips, nil
	}
	if f, ok := r.flights[host]; ok {
		r.mu.Unlock()
		<-f.done
		return f.ips, f.err
	}
	f := &flight{done: make(chan struct{})}
	r.flights[host] = f
	r.mu.Unlock()
	ips, err := safeResolve(r.resolve, ctx, host)
	r.mu.Lock()
	if err == nil {
		r.cache[host] = cacheEntry{ips: ips, at: time.Now()}
	}
	f.ips, f.err = ips, err
	delete(r.flights, host)
	close(f.done)
	r.mu.Unlock()
	return ips, err
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
	r.flights = make(map[string]*flight)
	r.mu.Unlock()
}

// Len 返回缓存条目数。
func (r *Resolver) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.cache)
}

// Inflight 返回正在解析的 host 数。
func (r *Resolver) Inflight() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.flights)
}

func safeResolve(fn func(ctx context.Context, network, host string) ([]net.IP, error), ctx context.Context, host string) (ips []net.IP, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			ips = nil
			err = fmt.Errorf("resolverx: panic: %v", rec)
		}
	}()
	return fn(ctx, "ip", host)
}
