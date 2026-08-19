// Package auditx 提供一个内存审计事件日志器。
// 每条事件包含 actor、动作、目标与附加上下文，按时间顺序记录。
package auditx

import (
	"sync"
	"sync/atomic"
	"time"
)

// Event 是单条审计事件。
type Event struct {
	Seq     uint64         `json:"seq"`
	At      time.Time      `json:"at"`
	Actor   string         `json:"actor"`
	Action  string         `json:"action"`
	Target  string         `json:"target"`
	Outcome string         `json:"outcome"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// Logger 是审计日志器。
type Logger struct {
	mu     sync.Mutex
	events []Event
	seq    atomic.Uint64
}

// New 创建一个 Logger。
func New() *Logger { return &Logger{} }

// Record 记录一条事件并返回它的序号。
func (l *Logger) Record(actor, action, target, outcome string, meta map[string]any) Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := Event{
		Seq:     l.seq.Add(1),
		At:      time.Now(),
		Actor:   actor,
		Action:  action,
		Target:  target,
		Outcome: outcome,
		Meta:    meta,
	}
	l.events = append(l.events, e)
	return e
}

// History 返回事件历史副本。
func (l *Logger) History() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Event, len(l.events))
	copy(out, l.events)
	return out
}

// Tail 返回最近 n 条事件。
func (l *Logger) Tail(n int) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n <= 0 || n > len(l.events) {
		n = len(l.events)
	}
	out := make([]Event, n)
	copy(out, l.events[len(l.events)-n:])
	return out
}

// Filter 按 actor 过滤历史。
func (l *Logger) Filter(actor string) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := []Event{}
	for _, e := range l.events {
		if e.Actor == actor {
			out = append(out, e)
		}
	}
	return out
}

// Stats 是事件统计。
type Stats struct {
	Total    int `json:"total"`
	Failures int `json:"failures"`
}

// Stats 返回总条数与失败条数。
func (l *Logger) Stats() Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(l.events)
	fails := 0
	for _, e := range l.events {
		if e.Outcome != "success" {
			fails++
		}
	}
	return Stats{Total: n, Failures: fails}
}

// Clear 清空日志。
func (l *Logger) Clear() {
	l.mu.Lock()
	l.events = nil
	l.mu.Unlock()
}
