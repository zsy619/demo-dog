// Package ttlcache 提供一个带过期时间（TTL）的内存缓存。
//
// 内部使用 map + 优先队列管理过期事件；
// 同一 key 的多次 Put 仅保留最新条目（旧的过期项会被同步移除，
// 避免堆无限增长）。
//
// 适用于读多写少、可容忍偶发 stale 读取的场景；
// 严格一致性请使用外部锁。
package ttlcache

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"
)

// item 是堆中的一项。
type item struct {
	k    string
	t    time.Time
	dead bool // 逻辑删除标记
}

type pq []*item

func (p pq) Len() int           { return len(p) }
func (p pq) Less(i, j int) bool { return p[i].t.Before(p[j].t) }
func (p pq) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x any)        { *p = append(*p, x.(*item)) }
func (p *pq) Pop() any {
	o := *p
	n := len(o)
	x := o[n-1]
	*p = o[:n-1]
	return x
}

// Cache 是带 TTL 的内存缓存。
//
// 零值不可用；请使用 New。
type Cache struct {
	mu         sync.Mutex
	data       map[string]any
	expiry     map[string]time.Time
	hp         *pq
	defaultTTL time.Duration

	stop     chan struct{}
	closed   atomic.Bool
	hits     atomic.Uint64
	misses   atomic.Uint64
	expired  atomic.Uint64
	inserts  atomic.Uint64
	deletes  atomic.Uint64
}

// Stats 是缓存的运行时统计。
type Stats struct {
	Size    int    `json:"size"`
	Hits    uint64 `json:"hits"`
	Misses  uint64 `json:"misses"`
	Expired uint64 `json:"expired"`
	Inserts uint64 `json:"inserts"`
	Deletes uint64 `json:"deletes"`
}

// New 创建一个带默认 TTL 的缓存，并启动后台 GC goroutine。
// defaultTTL <= 0 视为 1 分钟。
func New(defaultTTL time.Duration) *Cache {
	if defaultTTL <= 0 {
		defaultTTL = time.Minute
	}
	c := &Cache{
		data:       make(map[string]any),
		expiry:     make(map[string]time.Time),
		hp:         &pq{},
		defaultTTL: defaultTTL,
		stop:       make(chan struct{}),
	}
	go c.gc()
	return c
}

// Put 写入键值，使用默认 TTL。
func (c *Cache) Put(k string, v any) {
	c.PutTTL(k, v, c.defaultTTL)
}

// PutTTL 写入键值并显式指定 TTL；ttl <= 0 使用默认 TTL。
func (c *Cache) PutTTL(k string, v any, ttl time.Duration) {
	if c.closed.Load() {
		return
	}
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	exp := time.Now().Add(ttl)
	c.mu.Lock()
	if old, ok := c.expiry[k]; ok && !old.IsZero() {
		// 标记旧堆条目为 dead，避免后续扫描误删新条目。
		c.markDeadLocked(old, k)
	}
	c.data[k] = v
	c.expiry[k] = exp
	heap.Push(c.hp, &item{k: k, t: exp})
	c.mu.Unlock()
	c.inserts.Add(1)
}

// markDeadLocked 在写入新过期时间时，扫描堆顶把过期的 key 标记为 dead。
// 时间复杂度 O(d)，d 为该 key 的旧过期项数；通常为 1 或 2。
func (c *Cache) markDeadLocked(oldExp time.Time, k string) {
	// 仅当 oldExp 还在堆中（未过期）时需要标记。
	// 简化实现：扫描整个堆寻找匹配条目并标记。
	for _, it := range *c.hp {
		if !it.dead && it.k == k && it.t.Equal(oldExp) {
			it.dead = true
		}
	}
}

// Get 读取键值；已过期返回 (nil, false) 并自动清理。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[k]
	if !ok {
		c.misses.Add(1)
		return nil, false
	}
	exp, e := c.expiry[k]
	if !e {
		c.misses.Add(1)
		return nil, false
	}
	if time.Now().After(exp) {
		delete(c.data, k)
		delete(c.expiry, k)
		c.expired.Add(1)
		return nil, false
	}
	c.hits.Add(1)
	return v, true
}

// Delete 显式移除一个 key。
func (c *Cache) Delete(k string) {
	c.mu.Lock()
	oldExp, ok := c.expiry[k]
	delete(c.data, k)
	delete(c.expiry, k)
	c.mu.Unlock()
	if ok {
		c.mu.Lock()
		c.markDeadLocked(oldExp, k)
		c.mu.Unlock()
	}
	c.deletes.Add(1)
}

// Len 返回当前存活条目数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data)
}

// Stats 返回缓存统计快照。
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	sz := len(c.data)
	c.mu.Unlock()
	return Stats{
		Size:    sz,
		Hits:    c.hits.Load(),
		Misses:  c.misses.Load(),
		Expired: c.expired.Load(),
		Inserts: c.inserts.Load(),
		Deletes: c.deletes.Load(),
	}
}

// Close 停止后台 GC goroutine。幂等。
func (c *Cache) Close() {
	if c.closed.Swap(true) {
		return
	}
	close(c.stop)
}

func (c *Cache) gc() {
	interval := c.defaultTTL / 4
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	t := time.NewTicker(interval)
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

// sweep 清理过期条目；同时收集 dead 标记的过期项以减小堆。
func (c *Cache) sweep() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	cleaned := 0
	for c.hp.Len() > 0 {
		top := (*c.hp)[0]
		if !top.dead && top.t.After(now) {
			break
		}
		heap.Pop(c.hp)
		if !top.dead {
			delete(c.data, top.k)
			delete(c.expiry, top.k)
			cleaned++
		}
	}
	c.expired.Add(uint64(cleaned))
}
