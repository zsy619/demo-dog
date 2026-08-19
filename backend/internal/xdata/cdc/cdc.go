// Package cdc 提供一个内存的变更数据捕获通道。
// 它记录操作类型、键、值与时间戳，供下游订阅消费。
package cdc

import (
	"sync"
	"sync/atomic"
	"time"
)

// Op 是变更种类。
type Op int

const (
	OpPut Op = iota
	OpDelete
)

// Event 是单次变更事件。
type Event struct {
	Seq     uint64 `json:"seq"`
	Op      Op     `json:"op"`
	Key     string `json:"key"`
	Value   []byte `json:"value,omitempty"`
	At      time.Time `json:"at"`
}

// Recorder 是变更源：调用 Put/Delete 追加事件。
type Recorder struct {
	mu     sync.Mutex
	events []Event
	seq    atomic.Uint64
	subs   map[int]chan Event
	nextID int
	wg     sync.WaitGroup
	stop   chan struct{}
}

// New 创建一个 Recorder。
func New(buffer int) *Recorder {
	if buffer <= 0 {
		buffer = 16
	}
	return &Recorder{
		subs: make(map[int]chan Event),
		stop: make(chan struct{}),
	}
}

// Put 记录一个 Put 事件并广播给订阅者。
func (r *Recorder) Put(key string, value []byte) Event {
	return r.emit(Event{Op: OpPut, Key: key, Value: value})
}

// Delete 记录一个 Delete 事件。
func (r *Recorder) Delete(key string) Event {
	return r.emit(Event{Op: OpDelete, Key: key})
}

func (r *Recorder) emit(e Event) Event {
	r.mu.Lock()
	r.seq.Add(1)
	e.Seq = r.seq.Load()
	e.At = time.Now()
	r.events = append(r.events, e)
	subs := make([]chan Event, 0, len(r.subs))
	for _, c := range r.subs {
		subs = append(subs, c)
	}
	r.mu.Unlock()
	for _, c := range subs {
		select {
		case c <- e:
		default:
		}
	}
	return e
}

// Subscribe 注册一个订阅者。
func (r *Recorder) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan Event, buffer)
	r.mu.Lock()
	id := r.nextID
	r.nextID++
	r.subs[id] = ch
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		if c, ok := r.subs[id]; ok {
			delete(r.subs, id)
			close(c)
		}
		r.mu.Unlock()
	}
}

// History 返回已记录的事件副本。
func (r *Recorder) History() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

// Snapshot 是顺序统计信息。
type Snapshot struct {
	Seq     uint64 `json:"seq"`
	Events  int    `json:"events"`
	Subs    int    `json:"subs"`
}

// Stats 返回当前统计。
func (r *Recorder) Stats() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Snapshot{Seq: r.seq.Load(), Events: len(r.events), Subs: len(r.subs)}
}

// Tail 返回最近 n 个事件。
func (r *Recorder) Tail(n int) []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || n > len(r.events) {
		n = len(r.events)
	}
	out := make([]Event, n)
	copy(out, r.events[len(r.events)-n:])
	return out
}
