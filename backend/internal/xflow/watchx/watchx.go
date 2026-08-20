// Package watchx 提供简单的发布/订阅监视器。
package watchx

import "sync"

// Watcher 是多观察者订阅器。
type Watcher[T any] struct {
	mu      sync.RWMutex
	subs    []*sub[T]
}

type sub[T any] struct {
	ch     chan T
	active bool
}

// New 创建一个空 Watcher。
func New[T any]() *Watcher[T] { return &Watcher[T]{} }

// Subscribe 订阅事件，buffer 是事件缓冲（>0）。
// 返回 (recv chan, cancel func)。cancel 是幂等的。
func (w *Watcher[T]) Subscribe(buffer int) (<-chan T, func()) {
	if buffer < 1 {
		buffer = 1
	}
	s := &sub[T]{ch: make(chan T, buffer), active: true}
	w.mu.Lock()
	w.subs = append(w.subs, s)
	w.mu.Unlock()
	cancel := func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		if !s.active {
			return
		}
		s.active = false
		for i, ss := range w.subs {
			if ss == s {
				w.subs = append(w.subs[:i], w.subs[i+1:]...)
				close(s.ch)
				break
			}
		}
	}
	return s.ch, cancel
}

// Publish 发布一个事件（非阻塞）。
func (w *Watcher[T]) Publish(v T) {
	w.mu.RLock()
	snapshot := make([]*sub[T], len(w.subs))
	copy(snapshot, w.subs)
	w.mu.RUnlock()
	for _, s := range snapshot {
		select {
		case s.ch <- v:
		default:
		}
	}
}

// Subscribers 返回订阅者数量。
func (w *Watcher[T]) Subscribers() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.subs)
}
