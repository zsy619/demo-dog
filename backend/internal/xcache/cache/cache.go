// Package cache 通用内存缓存：支持 GetOrLoad、TTL、容量淘汰。
package cache

import (
	"errors"
	"sync"
	"time"
)

// Entry 表示一个缓存值。
type Entry struct {
	Value     any
	ExpiresAt time.Time
}

func (e *Entry) expired(now time.Time) bool {
	return !e.ExpiresAt.IsZero() && !now.Before(e.ExpiresAt)
}

// Cache 是一个简单的 TTL 缓存，支持 GetOrLoad singleflight 模式。
type Cache struct {
	mu       sync.RWMutex
	items    map[string]*Entry
	ttl      time.Duration
	maxItems int
	hits     uint64
	misses   uint64
	evicted  uint64
	expired  uint64
	loadMu   sync.Mutex
	inflight map[string]*struct {
		ch chan struct{}
		v  any
		e  error
	}
}

// Config 用于配置缓存。
type Config struct {
	TTL      time.Duration
	MaxItems int
}

// New 构造一个 Cache。
func New(cfg Config) *Cache {
	if cfg.MaxItems <= 0 {
		cfg.MaxItems = 1024
	}
	return &Cache{
		items:    make(map[string]*Entry),
		ttl:      cfg.TTL,
		maxItems: cfg.MaxItems,
		inflight: make(map[string]*struct {
			ch chan struct{}
			v  any
			e  error
		}),
	}
}

// Get 返回缓存值，未命中时返回 (nil, false)。
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		c.misses++
		return nil, false
	}
	if e.expired(time.Now()) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		c.expired++
		c.misses++
		return nil, false
	}
	c.hits++
	return e.Value, true
}

// Set 以默认 TTL 插入一个值。
func (c *Cache) Set(key string, v any) {
	c.SetTTL(key, v, c.ttl)
}

// SetTTL 以自定义 TTL 插入一个值。
func (c *Cache) SetTTL(key string, v any, ttl time.Duration) {
	c.mu.Lock()
	if _, ok := c.items[key]; !ok && len(c.items) >= c.maxItems {
		c.evictOne()
		c.evicted++
	}
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.items[key] = &Entry{Value: v, ExpiresAt: exp}
	c.mu.Unlock()
}

// Delete 删除一个 key。
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Len 返回当前存储的元素数量。
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats 是计数器的一份快照。
type Stats struct {
	Items   int    `json:"items"`
	Max     int    `json:"max"`
	Hits    uint64 `json:"hits"`
	Misses  uint64 `json:"misses"`
	Evicted uint64 `json:"evicted"`
	Expired uint64 `json:"expired"`
}

// Stats 返回 Stats 快照。
func (c *Cache) Stats() Stats {
	return Stats{
	Items:   c.Len(),
	Max:     c.maxItems,
	Hits:    c.hits,
	Misses:  c.misses,
	Evicted: c.evicted,
	Expired: c.expired,
	}
}

// ErrLoadInProgress 在已有其他协程正在为该 key 执行加载时返回。
var ErrLoadInProgress = errors.New("load already in progress")

// GetOrLoad 返回缓存值；若 key 不存在则调用 load()。
// 同一缺失 key 的并发调用共享一次 in-flight 加载，
// 除首个调用者外，其余调用者均返回 ErrLoadInProgress。
func (c *Cache) GetOrLoad(key string, load func() (any, error)) (any, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	c.loadMu.Lock()
	if inflight, ok := c.inflight[key]; ok {
		c.loadMu.Unlock()
		<-inflight.ch
		if inflight.e != nil {
			return nil, inflight.e
		}
		return inflight.v, nil
	}
	ch := make(chan struct{})
	c.inflight[key] = &struct {
		ch chan struct{}
		v  any
		e  error
	}{ch: ch}
	c.loadMu.Unlock()
	v, err := load()
	c.loadMu.Lock()
	inf := c.inflight[key]
	inf.v = v
	inf.e = err
	delete(c.inflight, key)
	close(ch)
	c.loadMu.Unlock()
	if err == nil {
		c.Set(key, v)
	}
	return v, err
}

// Flush 清空所有条目。
func (c *Cache) Flush() {
	c.mu.Lock()
	c.items = make(map[string]*Entry)
	c.mu.Unlock()
}

// evictOne 移除最近插入的 key（廉价的 LRU 近似；
// 足以应对小容量有界缓存）。
func (c *Cache) evictOne() {
	var newest string
	var newestAt time.Time
	for k, e := range c.items {
		if newest == "" || e.ExpiresAt.After(newestAt) {
			newest = k
			newestAt = e.ExpiresAt
		}
	}
	if newest != "" {
		delete(c.items, newest)
	}
}
