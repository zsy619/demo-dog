// Package pubsub 进程内 pub/sub：发布订阅模式，扇出消息。
package pubsub

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Message is one published payload.
type Message struct {
	Topic   string
	Payload []byte
}

// Bus is a topic-based pub/sub fan-out.
type Bus struct {
	mu     sync.RWMutex
	topics map[string]map[int]*Subscriber
	next   int
	pub    atomic.Uint64
	drop   atomic.Uint64
}

// Subscriber is one consumer with a bounded buffer.
type Subscriber struct {
	ch     chan Message
	dropped atomic.Uint64
	closed atomic.Bool
}

// NewBus creates an empty Bus.
func NewBus() *Bus {
	return &Bus{topics: make(map[string]map[int]*Subscriber)}
}

// ErrUnknownTopic is returned when Subscribe references a
// topic nobody has registered.
var ErrUnknownTopic = errors.New("unknown topic")

// Subscribe registers a subscriber for a topic.
func (b *Bus) Subscribe(topic string, bufferSize int) *Subscriber {
	if bufferSize <= 0 {
		bufferSize = 16
	}
	s := &Subscriber{ch: make(chan Message, bufferSize)}
	b.mu.Lock()
	if _, ok := b.topics[topic]; !ok {
		b.topics[topic] = make(map[int]*Subscriber)
	}
	b.topics[topic][b.next] = s
	b.next++
	b.mu.Unlock()
	return s
}

// Publish sends a message to all subscribers on the topic.
// Non-blocking: when a subscriber buffer is full, the
// message is dropped for that subscriber.
func (b *Bus) Publish(topic string, payload []byte) {
	b.mu.RLock()
	subs := make([]*Subscriber, 0, len(b.topics[topic]))
	for _, s := range b.topics[topic] {
		subs = append(subs, s)
	}
	b.mu.RUnlock()
	b.pub.Add(1)
	for _, s := range subs {
		if s.closed.Load() {
			continue
		}
		select {
		case s.ch <- Message{Topic: topic, Payload: payload}:
		default:
			s.dropped.Add(1)
			b.drop.Add(1)
		}
	}
}

// Messages returns the receive channel for the subscriber.
func (s *Subscriber) Messages() <-chan Message { return s.ch }

// Dropped returns the number of messages dropped for this
// subscriber.
func (s *Subscriber) Dropped() uint64 { return s.dropped.Load() }

// Close closes the subscriber.
func (s *Subscriber) Close() {
	if s.closed.CompareAndSwap(false, true) {
		close(s.ch)
	}
}

// Stats returns counters.
type Stats struct {
	Topics      int    `json:"topics"`
	Subscribers int    `json:"subscribers"`
	Published   uint64 `json:"published"`
	Dropped     uint64 `json:"dropped"`
}

// Stats returns the snapshot.
func (b *Bus) Stats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := 0
	for _, t := range b.topics {
		n += len(t)
	}
	return Stats{Topics: len(b.topics), Subscribers: n, Published: b.pub.Load(), Dropped: b.drop.Load()}
}

// CloseTopic removes all subscribers from a topic.
func (b *Bus) CloseTopic(topic string) {
	b.mu.Lock()
	subs := b.topics[topic]
	delete(b.topics, topic)
	b.mu.Unlock()
	for _, s := range subs {
		s.Close()
	}
}
