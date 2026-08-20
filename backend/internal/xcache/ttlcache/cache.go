package ttlcache

// cache.go:Cache 主体与 Stats 类型。
//
// Cache 通过 sync.Mutex 保护 data / expiry / hp;
// 通过 atomic.Uint64 统计 hits / misses / expired / inserts / deletes。
// 后台 goroutine 周期性调用 sweep 清理过期条目。

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"
)

// Cache 是带 TTL 的内存缓存。
//
// 零值不可用;请使用 New。
type Cache struct {
	mu         sync.Mutex          // 保护 data / expiry / hp
	data       map[string]any      // 键值存储
	expiry     map[string]time.Time // key → 过期时间
	hp         *pq                 // 过期事件堆(最小堆)
	defaultTTL time.Duration       // 默认 TTL

	stop    chan struct{}   // 通知后台 GC 退出
	closed  atomic.Bool     // 是否已关闭
	hits    atomic.Uint64   // 命中数
	misses  atomic.Uint64   // 未命中数
	expired atomic.Uint64   // 过期淘汰数
	inserts atomic.Uint64   // 写入数
	deletes atomic.Uint64   // 删除数
}

// Stats 是缓存的运行时统计。
type Stats struct {
	Size    int    `json:"size"`    // 当前存活条目数
	Hits    uint64 `json:"hits"`    // 命中次数
	Misses  uint64 `json:"misses"`  // 未命中次数
	Expired uint64 `json:"expired"` // 因过期被淘汰次数
	Inserts uint64 `json:"inserts"` // 写入次数
	Deletes uint64 `json:"deletes"` // 显式删除次数
}

// New 创建一个带默认 TTL 的缓存,并启动后台 GC goroutine。
//
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

// Put 写入键值,使用默认 TTL。
func (c *Cache) Put(k string, v any) {
	c.PutTTL(k, v, c.defaultTTL)
}

// PutTTL 写入键值并显式指定 TTL;ttl <= 0 使用默认 TTL。
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
		// 标记旧堆条目为 dead,避免后续扫描误删新条目。
		c.markDeadLocked(old, k)
	}
	c.data[k] = v
	c.expiry[k] = exp
	heap.Push(c.hp, &item{k: k, t: exp})
	c.mu.Unlock()
	c.inserts.Add(1)
}

// markDeadLocked 在写入新过期时间时,扫描堆顶把过期的 key 标记为 dead。
//
// 时间复杂度 O(d),d 为该 key 的旧过期项数;通常为 1 或 2。
func (c *Cache) markDeadLocked(oldExp time.Time, k string) {
	// 仅当 oldExp 还在堆中(未过期)时需要标记。
	// 简化实现:扫描整个堆寻找匹配条目并标记。
	for _, it := range *c.hp {
		if !it.dead && it.k == k && it.t.Equal(oldExp) {
			it.dead = true
		}
	}
}

// Get 读取键值;已过期返回 (nil, false) 并自动清理。
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

// gc 是后台周期性 sweep 循环。
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

// sweep 清理过期条目;同时收集 dead 标记的过期项以减小堆。
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
