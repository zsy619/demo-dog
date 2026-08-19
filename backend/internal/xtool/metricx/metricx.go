// Package metricx 提供一个轻量级指标推送器：
// 定期把指标快照以 Prometheus 文本格式写到 Writer。
package metricx

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Counter 是一个单调递增指标。
type Counter struct {
	val atomic.Uint64
}

// Inc 增加 1。
func (c *Counter) Inc() { c.val.Add(1) }

// Add 增加 n。
func (c *Counter) Add(n uint64) { c.val.Add(n) }

// Value 返回当前值。
func (c *Counter) Value() uint64 { return c.val.Load() }

// Pusher 维护一组命名指标。
type Pusher struct {
	mu       sync.RWMutex
	counters map[string]*Counter
}

// NewPusher 创建一个空 Pusher。
func NewPusher() *Pusher {
	return &Pusher{counters: make(map[string]*Counter)}
}

// Counter 取（或创建）一个 Counter。
func (p *Pusher) Counter(name string) *Counter {
	p.mu.RLock()
	c, ok := p.counters[name]
	p.mu.RUnlock()
	if ok {
		return c
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok = p.counters[name]; ok {
		return c
	}
	c = &Counter{}
	p.counters[name] = c
	return c
}

// WriteSnapshot 把所有指标以 Prometheus 文本格式写入 w。
func (p *Pusher) WriteSnapshot(w io.Writer) error {
	p.mu.RLock()
	names := make([]string, 0, len(p.counters))
	for k := range p.counters {
		names = append(names, k)
	}
	vals := make(map[string]uint64, len(p.counters))
	for k, c := range p.counters {
		vals[k] = c.Value()
	}
	p.mu.RUnlock()
	sort.Strings(names)
	ts := time.Now().Unix()
	for _, n := range names {
		if _, err := fmt.Fprintf(w, "# TYPE %s counter\n", n); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%s %d %d\n", n, vals[n], ts); err != nil {
			return err
		}
	}
	return nil
}

// Names 返回所有指标名。
func (p *Pusher) Names() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, 0, len(p.counters))
	for k := range p.counters {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Reset 清空所有指标。
func (p *Pusher) Reset() {
	p.mu.Lock()
	p.counters = make(map[string]*Counter)
	p.mu.Unlock()
}
