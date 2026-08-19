// Package runtimex 提供运行时错误与告警事件收集：
// 捕获 goroutine panic、上报运行时统计，作为诊断入口。
package runtimex

import (
	"sync"
	"sync/atomic"
	"time"
)

// Event 是一次运行时事件。
type Event struct {
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"`
	Message string    `json:"message"`
	Caller  string    `json:"caller,omitempty"`
}

// Recorder 收集运行时事件。
type Recorder struct {
	mu       sync.Mutex
	events   []Event
	capacity int
	panics   atomic.Uint64
	warns    atomic.Uint64
}

// New 创建一个容量为 capacity 的 Recorder。
func New(capacity int) *Recorder {
	if capacity <= 0 {
		capacity = 64
	}
	return &Recorder{capacity: capacity}
}

// RecordPanic 记录一次 panic 事件。
func (r *Recorder) RecordPanic(msg, caller string) {
	r.mu.Lock()
	r.append(Event{At: time.Now(), Kind: "panic", Message: msg, Caller: caller})
	r.mu.Unlock()
	r.panics.Add(1)
}

// RecordWarn 记录一次告警。
func (r *Recorder) RecordWarn(msg, caller string) {
	r.mu.Lock()
	r.append(Event{At: time.Now(), Kind: "warn", Message: msg, Caller: caller})
	r.mu.Unlock()
	r.warns.Add(1)
}

func (r *Recorder) append(e Event) {
	if len(r.events) >= r.capacity {
		r.events = r.events[1:]
	}
	r.events = append(r.events, e)
}

// History 返回所有事件副本。
func (r *Recorder) History() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

// Stats 返回计数视图。
type Stats struct {
	Panics   uint64 `json:"panics"`
	Warns    uint64 `json:"warns"`
	Events   int    `json:"events"`
	Capacity int    `json:"capacity"`
}

// Stats 返回当前计数。
func (r *Recorder) Stats() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Stats{Panics: r.panics.Load(), Warns: r.warns.Load(), Events: len(r.events), Capacity: r.capacity}
}

// Clear 清空事件。
func (r *Recorder) Clear() {
	r.mu.Lock()
	r.events = nil
	r.mu.Unlock()
}

// GoSafe 在独立 goroutine 中运行 fn，捕获 panic 并上报。
func (r *Recorder) GoSafe(fn func()) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				r.RecordPanic(recToString(rec), "GoSafe")
			}
		}()
		fn()
	}()
}

func recToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return "panic"
}
