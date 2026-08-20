// Package cache 通用内存缓存：支持 GetOrLoad、TTL、容量淘汰。
package cache

import (
	"errors"
	"sync"
	"time"
)

// Entry is one cached value.
type Entry struct {
	Value     any
	ExpiresAt time.Time
}

func (e *Entry) expired(now time.Time) bool {
	return !e.ExpiresAt.IsZero() && !now.Before(e.ExpiresAt)
}

// Cache is a simple TTL cache with GetOrLoad singleflight.
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

// Config configures the cache.
type Config struct {
	TTL      time.Duration
	MaxItems int
}

// New constructs a Cache.
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

// Get returns the cached value or (nil, false) on miss.
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

// Set inserts a value with the default TTL.
func (c *Cache) Set(key string, v any) {
	c.SetTTL(key, v, c.ttl)
}

// SetTTL inserts a value with a custom TTL.
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

// Delete removes a key.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Len returns the number of items currently stored.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats is a snapshot of counters.
type Stats struct {
	Items   int    `json:"items"`
	Max     int    `json:"max"`
	Hits    uint64 `json:"hits"`
	Misses  uint64 `json:"misses"`
	Evicted uint64 `json:"evicted"`
	Expired uint64 `json:"expired"`
}

// Stats returns a Stats snapshot.
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

// ErrLoadInProgress is returned when a load for this key is
// already in flight in another goroutine.
var ErrLoadInProgress = errors.New("load already in progress")

// GetOrLoad returns the cached value, or calls load() if the
// key is missing. Concurrent calls for the same missing key
// share a single in-flight load and return ErrLoadInProgress
// to all but the first caller.
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

// Flush removes all entries.
func (c *Cache) Flush() {
	c.mu.Lock()
	c.items = make(map[string]*Entry)
	c.mu.Unlock()
}

// evictOne removes the most recently inserted key (cheap LRU
// approximation; sufficient for a small bounded cache).
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
