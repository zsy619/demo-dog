// Package audit 提供不可变审计日志的内存存储与查询。
package audit

import (
	"encoding/json"
	"sync"
	"time"
)

// Event 是一个不可变审计事件。
type Event struct {
	Time    time.Time      `json:"time"`
	Actor   string         `json:"actor"`
	Action  string         `json:"action"`
	Target  string         `json:"target"`
	Result  string         `json:"result"`
	Detail  map[string]any `json:"detail,omitempty"`
}

// Marshal 返回 JSON 字符串。
func (e Event) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Log 是线程安全的审计日志存储。
type Log struct {
	mu     sync.RWMutex
	events []Event
}

// New 创建一个新的 Log。
func New() *Log {
	return &Log{}
}

// Append 追加一个事件。
func (l *Log) Append(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	l.mu.Lock()
	l.events = append(l.events, e)
	l.mu.Unlock()
}

// Recent 返回最近 n 条事件。
func (l *Log) Recent(n int) []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if n <= 0 || n > len(l.events) {
		n = len(l.events)
	}
	out := make([]Event, n)
	copy(out, l.events[len(l.events)-n:])
	return out
}

// Filter 按 actor/action 返回匹配的事件。
func (l *Log) Filter(actor, action string) []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Event, 0)
	for _, e := range l.events {
		if (actor == "" || e.Actor == actor) && (action == "" || e.Action == action) {
			out = append(out, e)
		}
	}
	return out
}

// Len 返回总条数。
func (l *Log) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.events)
}

// Clear 清空。
func (l *Log) Clear() {
	l.mu.Lock()
	l.events = l.events[:0]
	l.mu.Unlock()
}
