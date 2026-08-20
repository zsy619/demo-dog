// Package registry 服务注册表：按名字注册与查找服务实例。
package registry

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Entry 是一个类型化配置槽。
type Entry struct {
	Key    string
	Value  any
	Reason string
}

// Registry 是线程安全的类型化配置注册表。
type Registry struct {
	mu      sync.RWMutex
	data    map[string]any
	reasons map[string]string
	version atomic.Uint64
}

// New 创建一个空 Registry。
func New() *Registry {
	return &Registry{data: make(map[string]any), reasons: make(map[string]string)}
}

// Set 存储一个 key。
func (r *Registry) Set(key string, value any, reason string) {
	r.mu.Lock()
	r.data[key] = value
	r.reasons[key] = reason
	r.mu.Unlock()
	r.version.Add(1)
}

// Get returns the value for key.
func (r *Registry) Get(key string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.data[key]
	return v, ok
}

// Reason returns the reason for the last set.
func (r *Registry) Reason(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.reasons[key]
}

// Delete 移除一个 key。
func (r *Registry) Delete(key string) {
	r.mu.Lock()
	delete(r.data, key)
	delete(r.reasons, key)
	r.mu.Unlock()
	r.version.Add(1)
}

// Snapshot 返回所有条目的副本。
func (r *Registry) Snapshot() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]any, len(r.data))
	for k, v := range r.data {
		out[k] = v
	}
	return out
}

// Version 返回当前版本计数器。
func (r *Registry) Version() uint64 {
	return r.version.Load()
}

// ErrKeyMissing 在 key 缺失时返回。
var ErrKeyMissing = errors.New("key missing")

// ErrBadType 在存储值不是预期
// the expected type.
var ErrBadType = errors.New("bad type")

// GetString 将值作为 string 返回。
func (r *Registry) GetString(key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.data[key]
	if !ok {
		return "", ErrKeyMissing
	}
	s, ok := v.(string)
	if !ok {
		return "", ErrBadType
	}
	return s, nil
}

// GetInt 将值作为 int 返回。
func (r *Registry) GetInt(key string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.data[key]
	if !ok {
		return 0, ErrKeyMissing
	}
	switch n := v.(type) {
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case float64:
		return int(n), nil
	}
	return 0, ErrBadType
}

// GetBool 将值作为 bool 返回。
func (r *Registry) GetBool(key string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.data[key]
	if !ok {
		return false, ErrKeyMissing
	}
	b, ok := v.(bool)
	if !ok {
		return false, ErrBadType
	}
	return b, nil
}

// Keys 返回所有排序后的 key。
func (r *Registry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.data))
	for k := range r.data {
		out = append(out, k)
	}
	return out
}
