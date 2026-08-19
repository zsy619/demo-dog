// Package orderedmap 提供保留插入顺序的线程安全 map，
// 适用于序列化输出需要稳定顺序的场景。
package orderedmap

import "sync"

// Ordered 是一个线程安全的有序 map。
type Ordered struct {
	mu   sync.RWMutex
	keys []string
	data map[string]any
}

// New 创建一个空 Ordered。
func New() *Ordered {
	return &Ordered{data: make(map[string]any)}
}

// Set 设置一个键值对。
func (o *Ordered) Set(k string, v any) {
	o.mu.Lock()
	if _, ok := o.data[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.data[k] = v
	o.mu.Unlock()
}

// Get 返回值与是否存在标志。
func (o *Ordered) Get(k string) (any, bool) {
	o.mu.RLock()
	v, ok := o.data[k]
	o.mu.RUnlock()
	return v, ok
}

// Delete 移除一个键。
func (o *Ordered) Delete(k string) {
	o.mu.Lock()
	if _, ok := o.data[k]; ok {
		delete(o.data, k)
		for i, key := range o.keys {
			if key == k {
				o.keys = append(o.keys[:i], o.keys[i+1:]...)
				break
			}
		}
	}
	o.mu.Unlock()
}

// Len 返回元素数。
func (o *Ordered) Len() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.keys)
}

// Keys 返回按插入顺序排列的键副本。
func (o *Ordered) Keys() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]string, len(o.keys))
	copy(out, o.keys)
	return out
}

// Range 按顺序遍历 kv，回调返回 false 时停止。
func (o *Ordered) Range(fn func(k string, v any) bool) {
	o.mu.RLock()
	for _, k := range o.keys {
		v := o.data[k]
		if !fn(k, v) {
			break
		}
	}
	o.mu.RUnlock()
}

// Clear 清空。
func (o *Ordered) Clear() {
	o.mu.Lock()
	o.keys = o.keys[:0]
	o.data = make(map[string]any)
	o.mu.Unlock()
}
