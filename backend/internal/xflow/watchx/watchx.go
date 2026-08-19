// Package watchx 提供简单的发布/订阅监视器。
package watchx

import "sync"

// Watcher 是多观察者订阅器。
type Watcher[T any] struct {
	mu   sync.RWMutex
	subs []chan T
}

// New 创建一个空 Watcher。
func New[T any]() *Watcher[T] { return &Watcher[T]{} }

// Subscribe 订阅事件，buffer 是事件缓冲（>0）。
func (w *Watcher[T]) Subscribe(buffer int) (<-chan T, func()) {
	if buffer < 1 {
		buffer = 1
	}
	ch := make(chan T, buffer)
	w.mu.Lock()
	w.subs = append(w.subs, ch)
	w.mu.Unlock()
	cancel := func() {
		w.mu.Lock()
		for i, c := range w.subs {
			if c == ch {
				w.subs = append(w.subs[:i], w.subs[i+1:]...)
				close(ch)
				break
			}
		}
		w.mu.Unlock()
	}
	return ch, cancel
}

// Publish 发布一个事件（非阻塞）。
func (w *Watcher[T]) Publish(v T) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, ch := range w.subs {
		select {
		case ch <- v:
		default:
			// 缓冲区满则丢弃
		}
	}
}

// Subscribers 返回订阅者数量。
func (w *Watcher[T]) Subscribers() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.subs)
}
