// Package broadcast 提供多接收者广播：订阅 + 发布模式。
package broadcast

import (
	"sync"
)

// Broadcaster 是一个泛型广播器。
type Broadcaster[T any] struct {
	mu  sync.Mutex
	subs map[uint64]chan T
	next uint64
}

// New 创建一个空的 Broadcaster。
func New[T any]() *Broadcaster[T] {
	return &Broadcaster[T]{subs: make(map[uint64]chan T)}
}

// Subscribe 返回一个 channel 与取消函数。
func (b *Broadcaster[T]) Subscribe(buf int) (<-chan T, func()) {
	if buf < 1 {
		buf = 16
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	id := b.next
	ch := make(chan T, buf)
	b.subs[id] = ch
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
		close(ch)
	}
}

// Publish 向所有订阅者发送事件；阻塞消费者会被丢弃策略跳过。
func (b *Broadcaster[T]) Publish(v T) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, ch := range b.subs {
		select {
		case ch <- v:
			n++
		default:
		}
	}
	return n
}

// Subs 返回订阅者数量。
func (b *Broadcaster[T]) Subs() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}
