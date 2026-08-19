// Package notify 提供主题无关的事件总线。
// 它允许多个订阅者按主题注册监听，发布时按主题扇出。
package notify

import (
	"errors"
	"sync"
)

// Event 是总线中流通的事件。
type Event struct {
	Topic   string
	Payload any
}

// ErrClosed 在关闭后发布时返回。
var ErrClosed = errors.New("notify: 已关闭")

// Bus 是事件总线。
type Bus struct {
	mu       sync.RWMutex
	subs     map[string]map[int]chan Event
	nextID   int
	closed   bool
	totalIn  int
	totalOut int
	dropped  int
}

// New 创建一个空 Bus。
func New() *Bus {
	return &Bus{subs: make(map[string]map[int]chan Event)}
}

// Subscribe 注册一个主题订阅，buffer 是订阅者通道的容量。
func (b *Bus) Subscribe(topic string, buffer int) (<-chan Event, func()) {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan Event, buffer)
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	if _, ok := b.subs[topic]; !ok {
		b.subs[topic] = make(map[int]chan Event)
	}
	b.subs[topic][id] = ch
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if m, ok := b.subs[topic]; ok {
			if c, ok := m[id]; ok {
				delete(m, id)
				close(c)
			}
			if len(m) == 0 {
				delete(b.subs, topic)
			}
		}
		b.mu.Unlock()
	}
}

// Publish 向主题的所有订阅者推送事件。非阻塞。
func (b *Bus) Publish(topic string, payload any) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ErrClosed
	}
	b.totalIn++
	subs := make([]chan Event, 0, len(b.subs[topic]))
	for _, c := range b.subs[topic] {
		subs = append(subs, c)
	}
	b.mu.Unlock()
	for _, c := range subs {
		ev := Event{Topic: topic, Payload: payload}
		select {
		case c <- ev:
			b.mu.Lock()
			b.totalOut++
			b.mu.Unlock()
		default:
			b.mu.Lock()
			b.dropped++
			b.mu.Unlock()
		}
	}
	return nil
}

// Stats 是总线计数器。
type Stats struct {
	TotalIn  int `json:"total_in"`
	TotalOut int `json:"total_out"`
	Dropped  int `json:"dropped"`
	Topics   int `json:"topics"`
}

// Stats 返回当前计数器快照。
func (b *Bus) Stats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return Stats{
		TotalIn:  b.totalIn,
		TotalOut: b.totalOut,
		Dropped:  b.dropped,
		Topics:   len(b.subs),
	}
}

// Close 关闭总线，关闭所有订阅者通道。
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, m := range b.subs {
		for _, c := range m {
			close(c)
		}
	}
	b.subs = nil
}
