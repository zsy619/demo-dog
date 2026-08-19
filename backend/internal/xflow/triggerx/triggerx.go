// Package triggerx 提供事件触发器：注册 handler，触发时按添加顺序同步调用。
package triggerx

import "sync"

// Trigger 是一个泛型事件触发器。
type Trigger[T any] struct {
	mu      sync.RWMutex
	handlers []func(T)
}

// New 创建一个 Trigger。
func New[T any]() *Trigger[T] { return &Trigger[T]{} }

// Add 注册一个 handler，返回取消注册函数。
func (t *Trigger[T]) Add(h func(T)) func() {
	t.mu.Lock()
	t.handlers = append(t.handlers, h)
	idx := len(t.handlers) - 1
	t.mu.Unlock()
	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if idx < len(t.handlers) {
			t.handlers = append(t.handlers[:idx], t.handlers[idx+1:]...)
		}
	}
}

// Fire 触发所有 handler。
func (t *Trigger[T]) Fire(v T) {
	t.mu.RLock()
	handlers := make([]func(T), len(t.handlers))
	copy(handlers, t.handlers)
	t.mu.RUnlock()
	for _, h := range handlers {
		h(v)
	}
}

// Count 返回 handler 数。
func (t *Trigger[T]) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.handlers)
}

// Clear 清空所有 handler。
func (t *Trigger[T]) Clear() {
	t.mu.Lock()
	t.handlers = nil
	t.mu.Unlock()
}
