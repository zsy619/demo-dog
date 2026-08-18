// Package stream implements a tiny in-memory pub/sub used to push
// real-time events from the ingest pipeline to connected websocket clients.
//
// In a real Collector the same role is played by a metrics export pipeline
// or a tail of the Storage layer, but the demo just broadcasts every accepted
// payload so the frontend can show a live feed.
//
// We deliberately use the standard library only (no gorilla/websocket) to keep
// the dependency footprint small. The wire protocol is small text frames.
package stream

import (
	"sync"
	"time"
)

// Event is a single broadcast message.
type Event struct {
	Kind      string `json:"kind"`      // log|metric|span|service
	Service   string `json:"service"`
	Timestamp int64  `json:"timestamp"` // ms
	Body      string `json:"body,omitempty"`
	Value     float64 `json:"value,omitempty"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
}

// Hub is a thread-safe broadcaster with a bounded subscriber list.
type Hub struct {
	mu     sync.RWMutex
	subs   map[chan Event]struct{}
	closed bool
}

// NewHub returns an empty Hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[chan Event]struct{})}
}

// Subscribe returns a channel that receives every future event until
// the caller invokes the returned cancel function.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		close(ch)
		return ch, func() {}
	}
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
	}
	return ch, cancel
}

// Publish broadcasts an event to all subscribers. Slow consumers are dropped
// (their channel is full) so the publisher never blocks.
func (h *Hub) Publish(e Event) {
	if e.Timestamp == 0 {
		e.Timestamp = time.Now().UnixMilli()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
			// drop slow consumer
		}
	}
}

// SubscriberCount returns the number of active subscribers.
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

// Close unsubscribes all subscribers.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.subs {
		close(ch)
	}
	h.subs = map[chan Event]struct{}{}
}
