// Package qcache 提供按 query key 缓存函数调用结果。
// 使用 xflow/singleflight 合并并发请求，避免缓存击穿。
package qcache

import (
	"fmt"
	"sync"

	"github.com/zsy619/demo-dog/backend/internal/xflow/singleflight"
)

// Loader 加载缓存未命中时的数据。
type Loader[K comparable, V any] func(K) (V, error)

// Cache 泛型查询缓存。
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	items    map[K]V
	loader   Loader[K, V]
	inflight *singleflight.Group[K, V]
}

// New 创建一个查询缓存。
func New[K comparable, V any](loader Loader[K, V]) *Cache[K, V] {
	return &Cache[K, V]{
		items:    make(map[K]V),
		loader:   loader,
		inflight: singleflight.New[K, V](),
	}
}

// Get 查询，命中返回缓存，否则调用 loader 并写入。
// 同一 key 的并发请求会复用同一次 loader 调用。
func (c *Cache[K, V]) Get(k K) (V, error) {
	c.mu.Lock()
	if v, ok := c.items[k]; ok {
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()
	v, err := c.inflight.Do(k, func() (V, error) {
		// 再次检查缓存（在 flight 等待期间可能已被填入）
		c.mu.Lock()
		if vv, ok := c.items[k]; ok {
			c.mu.Unlock()
			return vv, nil
		}
		c.mu.Unlock()
		return safeCall(c.loader, k)
	})
	if err == nil {
		c.mu.Lock()
		c.items[k] = v
		c.mu.Unlock()
	}
	return v, err
}

// safeCall 在 loader panic 时返回零值与错误，避免泄漏 flight。
func safeCall[K comparable, V any](loader Loader[K, V], k K) (v V, err error) {
	defer func() {
		if r := recover(); r != nil {
			var z V
			v = z
			err = fmt.Errorf("qcache: loader panic: %v", r)
		}
	}()
	return loader(k)
}

// Invalidate 删除一个 key。
func (c *Cache[K, V]) Invalidate(k K) {
	c.mu.Lock()
	delete(c.items, k)
	c.mu.Unlock()
}

// Len 返回元素数。
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Inflight 返回正在加载的 key 数。
func (c *Cache[K, V]) Inflight() int {
	return c.inflight.Inflight()
}

// Clear 清空。
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	c.items = make(map[K]V)
	c.mu.Unlock()
}
