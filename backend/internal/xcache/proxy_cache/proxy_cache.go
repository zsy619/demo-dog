// Package proxy_cache 提供本地+远程的二层缓存：
// 查询时先查本地，未命中再查远程并回填本地。
// 使用 singleflight 防止同 key 的并发请求触发多次远程查询。
package proxy_cache

import (
	"sync"
	"sync/atomic"

	"github.com/zsy619/demo-dog/backend/internal/xflow/singleflight"
)

// Local 是本地缓存接口（QCache 等可适配）。
type Local interface {
	Get(k string) (string, bool)
	Put(k, v string)
}

// Remote 是远程缓存接口（用户自定义）。
type Remote interface {
	Get(k string) (string, error)
	Put(k, v string) error
}

// Proxy 是 Local + Remote 组合。
type Proxy struct {
	mu     sync.Mutex
	local  Local
	remote Remote
	hits   atomic.Int64
	misses atomic.Int64
	flight *singleflight.Group[string, string]
}

// New 创建 Proxy。
func New(l Local, r Remote) *Proxy {
	return &Proxy{local: l, remote: r, flight: singleflight.New[string, string]()}
}

// Get 查询：先 local，后 remote（带 singleflight 合并）。
func (p *Proxy) Get(k string) (string, bool) {
	if v, ok := p.local.Get(k); ok {
		p.hits.Add(1)
		return v, true
	}
	v, err := p.flight.Do(k, func() (string, error) {
		// 双重检查（等待期间可能已被其他请求回填）
		if vv, ok := p.local.Get(k); ok {
			return vv, nil
		}
		return p.remote.Get(k)
	})
	if err != nil || v == "" {
		p.misses.Add(1)
		return "", false
	}
	p.local.Put(k, v)
	p.misses.Add(1)
	return v, true
}

// Put 同时写入 local 与 remote。
func (p *Proxy) Put(k, v string) error {
	p.local.Put(k, v)
	return p.remote.Put(k, v)
}

// Stats 返回命中统计。
func (p *Proxy) Stats() (hits, misses int64) {
	return p.hits.Load(), p.misses.Load()
}

// ResetStats 重置命中统计。
func (p *Proxy) ResetStats() {
	p.hits.Store(0)
	p.misses.Store(0)
}

// Inflight 返回正在进行的远程查询数。
func (p *Proxy) Inflight() int {
	return p.flight.Inflight()
}
