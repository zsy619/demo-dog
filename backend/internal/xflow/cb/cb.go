// Package cb 提供通用回调注册中心。
package cb

import "sync"

// Callback 是一个泛型回调函数类型。
type Callback[T any] func(T)

// Registry 是一个线程安全的回调列表。
type Registry[T any] struct {
	mu     sync.RWMutex
	cbs    []Callback[T]
	nextID uint64
	index  map[uint64]int
}

// New 创建一个回调注册中心。
func New[T any]() *Registry[T] {
	return &Registry[T]{index: make(map[uint64]int)}
}

// Register 注册回调并返回 id。
func (r *Registry[T]) Register(cb Callback[T]) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	id := r.nextID
	r.cbs = append(r.cbs, cb)
	r.index[id] = len(r.cbs) - 1
	return id
}

// Unregister 通过 id 注销回调。
func (r *Registry[T]) Unregister(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	idx, ok := r.index[id]
	if !ok {
		return
	}
	r.cbs[idx] = nil
	delete(r.index, id)
}

// Trigger 调用所有回调。
func (r *Registry[T]) Trigger(v T) {
	r.mu.RLock()
	cbs := make([]Callback[T], len(r.cbs))
	copy(cbs, r.cbs)
	r.mu.RUnlock()
	for _, cb := range cbs {
		if cb != nil {
			cb(v)
		}
	}
}

// Count 返回当前回调数。
func (r *Registry[T]) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.index)
}

// Clear 清空所有回调。
func (r *Registry[T]) Clear() {
	r.mu.Lock()
	r.cbs = nil
	r.index = make(map[uint64]int)
	r.mu.Unlock()
}
