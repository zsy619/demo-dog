// Package registry 服务注册表：按名字注册与查找服务实例。
package registry

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Entry is one typed config slot.
type Entry struct {
	Key    string
	Value  any
	Reason string
}

// Registry is a thread-safe typed config registry.
type Registry struct {
	mu      sync.RWMutex
	data    map[string]any
	reasons map[string]string
	version atomic.Uint64
}

// New creates an empty Registry.
func New() *Registry {
	return &Registry{data: make(map[string]any), reasons: make(map[string]string)}
}

// Set stores a key.
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

// Delete removes a key.
func (r *Registry) Delete(key string) {
	r.mu.Lock()
	delete(r.data, key)
	delete(r.reasons, key)
	r.mu.Unlock()
	r.version.Add(1)
}

// Snapshot returns a copy of all entries.
func (r *Registry) Snapshot() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]any, len(r.data))
	for k, v := range r.data {
		out[k] = v
	}
	return out
}

// Version returns the current version counter.
func (r *Registry) Version() uint64 {
	return r.version.Load()
}

// ErrKeyMissing is returned when the key is absent.
var ErrKeyMissing = errors.New("key missing")

// ErrBadType is returned when the stored value is not of
// the expected type.
var ErrBadType = errors.New("bad type")

// GetString returns the value as a string.
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

// GetInt returns the value as an int.
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

// GetBool returns the value as a bool.
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

// Keys returns all keys sorted.
func (r *Registry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.data))
	for k := range r.data {
		out = append(out, k)
	}
	return out
}
