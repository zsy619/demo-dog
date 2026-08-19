// Package sortedkv 提供一个简单的有序 KV 容器（基于 sort）。
package sortedkv

import (
	"sort"
	"sync"
)

// KV 是一个有序 KV 容器。
type KV struct {
	mu sync.RWMutex
	m  map[string]any
}

// New 创建一个有序 KV。
func New() *KV { return &KV{m: make(map[string]any)} }

// Set 设置键值。
func (k *KV) Set(key string, v any) {
	k.mu.Lock()
	k.m[key] = v
	k.mu.Unlock()
}

// Get 读取键值。
func (k *KV) Get(key string) (any, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	v, ok := k.m[key]
	return v, ok
}

// Delete 删除键值。
func (k *KV) Delete(key string) {
	k.mu.Lock()
	delete(k.m, key)
	k.mu.Unlock()
}

// SortedKeys 按升序返回所有键。
func (k *KV) SortedKeys() []string {
	k.mu.RLock()
	keys := make([]string, 0, len(k.m))
	for key := range k.m {
		keys = append(keys, key)
	}
	k.mu.RUnlock()
	sort.Strings(keys)
	return keys
}

// Range 按排序顺序遍历键值对。
func (k *KV) Range(fn func(key string, value any) bool) {
	for _, key := range k.SortedKeys() {
		k.mu.RLock()
		v := k.m[key]
		k.mu.RUnlock()
		if !fn(key, v) {
			return
		}
	}
}

// Len 返回元素数。
func (k *KV) Len() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.m)
}
