// Package triggerx 提供事件触发器：注册 handler，触发时按添加顺序同步调用。
package triggerx

import "sync"

// Trigger 是一个泛型事件触发器。
type Trigger[T any] struct {
	mu       sync.RWMutex
	handlers []*handlerSlot[T]
}

type handlerSlot[T any] struct {
	handler func(T)
	active  bool
}

// New 创建一个 Trigger。
func New[T any]() *Trigger[T] { return &Trigger[T]{} }

// Add 注册一个 handler，返回取消注册函数。
// 取消函数是幂等的：多次调用安全。
func (t *Trigger[T]) Add(h func(T)) func() {
	slot := &handlerSlot[T]{handler: h, active: true}
	t.mu.Lock()
	t.handlers = append(t.handlers, slot)
	t.mu.Unlock()
	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if slot.active {
			slot.active = false
			// 从 slice 中移除
			for i, s := range t.handlers {
				if s == slot {
					t.handlers = append(t.handlers[:i], t.handlers[i+1:]...)
					break
				}
			}
		}
	}
}

// Fire 触发所有 handler。
func (t *Trigger[T]) Fire(v T) {
	t.mu.RLock()
	handlers := make([]*handlerSlot[T], len(t.handlers))
	copy(handlers, t.handlers)
	t.mu.RUnlock()
	for _, s := range handlers {
		s.handler(v)
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
