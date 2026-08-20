package api

import (
	"container/list"
	"sync"
	"time"
)

// LRUCache is a thread-safe, in-process least-recently-used cache
// with an optional per-entry TTL. It is intentionally tiny (a few
// hundred lines) and stdlib-only so the demo-dog collector does not
// pull in any new dependencies.
//
// Use cases:
//   * Cache for `/api/v1/query` (PromQL) responses.
//   * Cache for `/api/services` when nothing has changed.
//   * Cache for `/api/admin/keys` which changes very rarely.
//
// The cache is bounded by both a maximum entry count and an
// optional global byte budget. When either is exceeded, the least
// recently used entry is evicted.
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

// NewLRUCache returns a fresh cache.
//   cap: maximum number of entries (0 = 1024)
//   maxBytes: total byte budget (0 = unlimited)
//   ttl: per-entry TTL (0 = no expiry)
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

// Get returns the cached value and reports whether it was found.
// A value older than the TTL is treated as a miss.
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

// Set inserts or replaces an entry. The value is copied so the
// caller can mutate its slice after the call.
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

// Delete removes a key from the cache.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
}

// Len returns the number of cached entries.
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// LRUStats is a small summary suitable for /api/health.
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

// evict removes oldest entries until we are within both cap and
// maxBytes budgets.
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
