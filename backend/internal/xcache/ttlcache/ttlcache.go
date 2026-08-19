// Package ttlcache 提供一个简单的带过期时间（TTL）的内存缓存。
// 内部使用 map + 优先队列管理过期事件。
package ttlcache

import (
	"container/heap"
	"sync"
	"time"
)

// Cache 是带 TTL 的内存缓存。
type Cache struct {
	mu       sync.Mutex
	data     map[string]any
	expiry   map[string]time.Time
	hp       *pq
	halted   chan struct{}
	closed   bool
	stop     chan struct{}
	defaultTTL time.Duration
}

type item struct {
	k string
	t time.Time
}

type pq []*item

func (p pq) Len() int            { return len(p) }
func (p pq) Less(i, j int) bool  { return p[i].t.Before(p[j].t) }
func (p pq) Swap(i, j int)       { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x any)         { *p = append(*p, x.(*item)) }
func (p *pq) Pop() any {
	o := *p
	n := len(o)
	x := o[n-1]
	*p = o[:n-1]
	return x
}

// New 创建一个带默认 TTL 的缓存。
func New(defaultTTL time.Duration) *Cache {
	if defaultTTL <= 0 {
		defaultTTL = time.Minute
	}
	c := &Cache{
		data:       make(map[string]any),
		expiry:     make(map[string]time.Time),
		hp:         &pq{},
		halted:     make(chan struct{}),
		stop:       make(chan struct{}),
		defaultTTL: defaultTTL,
	}
	go c.gc()
	return c
}

// Put 写入键值，使用默认 TTL。
func (c *Cache) Put(k string, v any) {
	c.PutTTL(k, v, c.defaultTTL)
}

// PutTTL 写入键值并显式指定 TTL。
func (c *Cache) PutTTL(k string, v any, ttl time.Duration) {
	c.mu.Lock()
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	c.data[k] = v
	c.expiry[k] = time.Now().Add(ttl)
	heap.Push(c.hp, &item{k: k, t: c.expiry[k]})
	c.mu.Unlock()
}

// Get 读取键值；已过期则返回 false。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[k]
	if !ok {
		return nil, false
	}
	if exp, e := c.expiry[k]; e && time.Now().After(exp) {
		delete(c.data, k)
		delete(c.expiry, k)
		return nil, false
	}
	return v, true
}

// Delete 显式移除一个 key。
func (c *Cache) Delete(k string) {
	c.mu.Lock()
	delete(c.data, k)
	delete(c.expiry, k)
	c.mu.Unlock()
}

// Len 返回当前条目数（包含已过期但未回收的）。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data)
}

// Close 停止后台 GC。
func (c *Cache) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()
	close(c.stop)
}

func (c *Cache) gc() {
	t := time.NewTicker(c.defaultTTL / 4)
	if c.defaultTTL/4 < 100*time.Millisecond {
		t = time.NewTicker(100 * time.Millisecond)
	}
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			c.sweep()
		}
	}
}

func (c *Cache) sweep() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.hp.Len() > 0 {
		top := (*c.hp)[0]
		if top.t.After(now) {
			break
		}
		heap.Pop(c.hp)
		delete(c.data, top.k)
		delete(c.expiry, top.k)
	}
}
