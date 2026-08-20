package api

import (
	"container/list"
	"sync"
	"time"
)

// LRUCache 是线程安全的进程内最近最少使用缓存，
// 可选地支持按条目的 TTL。它刻意保持小巧（几百行）
// 且仅依赖标准库，以便 demo-dog 采集器
// 不引入任何新依赖。
//
// 使用场景：
//   * 缓存 /api/v1/query（PromQL）的响应。
//   * 在没有变化时缓存 /api/services。
//   * 缓存 /api/admin/keys（很少变化）。
//
// 缓存同时受最大条目数和可选的全局字节预算约束。
// 当任一约束被突破时，最久未使用的条目会被驱逐。
// 
type LRUCache struct {
	mu       sync.Mutex
	cap      int
	maxBytes int64
	ttl      time.Duration
	items    map[string]*list.Element
	order    *list.List
	hits     uint64
	misses   uint64
	evicts   uint64
	expired  uint64
	bytes    int64
}

type lruEntry struct {
	key     string
	value   []byte
	size    int64
	expires time.Time
}

// NewLRUCache 返回一个新的缓存。
//   cap: 最大条目数（0 = 1024）
//   maxBytes: 总字节预算（0 = 无限制）
//   ttl: 按条目的 TTL（0 = 不过期）
func NewLRUCache(cap int, maxBytes int64, ttl time.Duration) *LRUCache {
	if cap <= 0 {
		cap = 1024
	}
	return &LRUCache{
		cap:      cap,
		maxBytes: maxBytes,
		ttl:      ttl,
		items:    make(map[string]*list.Element, cap),
		order:    list.New(),
	}
}

// Get 返回缓存的值并报告是否命中。
// 超过 TTL 的值会被视为未命中。
func (c *LRUCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		c.misses++
		return nil, false
	}
	ent := el.Value.(*lruEntry)
	if c.ttl > 0 && time.Now().After(ent.expires) {
		c.removeElement(el)
		c.expired++
		c.misses++
		return nil, false
	}
	c.order.MoveToFront(el)
	c.hits++
	out := make([]byte, len(ent.value))
	copy(out, ent.value)
	return out, true
}

// Set 插入或替换一个条目。值会被拷贝，
// 因此调用方可以在调用后修改其切片。
func (c *LRUCache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		ent := el.Value.(*lruEntry)
		c.bytes -= ent.size
		ent.size = int64(len(value))
		c.bytes += ent.size
		ent.value = append([]byte(nil), value...)
		c.order.MoveToFront(el)
		c.setExpiry(ent)
		return
	}
	ent := &lruEntry{
		key:   key,
		value: append([]byte(nil), value...),
		size:  int64(len(value)),
	}
	c.setExpiry(ent)
	el := c.order.PushFront(ent)
	c.items[key] = el
	c.bytes += ent.size
	c.evict()
}

// Delete 从缓存中移除指定键。
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
}

// Len 返回缓存中的条目数。
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// LRUStats 是适合 /api/health 展示的小摘要。
type LRUStats struct {
	Size       int     `json:"size"`
	Capacity   int     `json:"capacity"`
	Bytes      int64   `json:"bytes"`
	MaxBytes   int64   `json:"max_bytes"`
	Hits       uint64  `json:"hits"`
	Misses     uint64  `json:"misses"`
	Evictions  uint64  `json:"evictions"`
	Expired    uint64  `json:"expired"`
	TTLSeconds float64 `json:"ttl_seconds"`
}

func (c *LRUCache) Stats() LRUStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return LRUStats{
		Size:       c.order.Len(),
		Capacity:   c.cap,
		Bytes:      c.bytes,
		MaxBytes:   c.maxBytes,
		Hits:       c.hits,
		Misses:     c.misses,
		Evictions:  c.evicts,
		Expired:    c.expired,
		TTLSeconds: c.ttl.Seconds(),
	}
}

func (c *LRUCache) removeElement(el *list.Element) {
	ent := el.Value.(*lruEntry)
	c.bytes -= ent.size
	c.order.Remove(el)
	delete(c.items, ent.key)
}

// evict 移除最旧的条目，直到同时满足 cap 和
// maxBytes 预算为止。
func (c *LRUCache) evict() {
	for c.order.Len() > c.cap {
		el := c.order.Back()
		if el == nil {
			return
		}
		c.removeElement(el)
		c.evicts++
	}
	for c.maxBytes > 0 && c.bytes > c.maxBytes {
		el := c.order.Back()
		if el == nil {
			return
		}
		c.removeElement(el)
		c.evicts++
	}
}

func (c *LRUCache) setExpiry(ent *lruEntry) {
	if c.ttl > 0 {
		ent.expires = time.Now().Add(c.ttl)
	} else {
		ent.expires = time.Time{}
	}
}
