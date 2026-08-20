package slo

// tracker.go：Tracker 主体与所有公共方法。
//
// Tracker 按 SLO 聚合样本并产出 Status（含当前成功率、剩余错误预算）。
// 使用 sync.Mutex 保护 defs / events；使用 atomic.Uint64 统计 alerts / reports。

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Event 是 Sample 在 Tracker 内部的存储形态。
type Event struct {
	Success bool          // 请求是否成功
	Took    time.Duration // 请求耗时
	At      time.Time     // 采样时间
}

// Tracker 按 SLO 聚合样本并产出 Status。
//
// 线程安全：所有方法都使用 sync.Mutex 保护 defs / events；
// alerts / reports 使用 atomic.Uint64。
type Tracker struct {
	mu      sync.Mutex           // 保护 defs / events
	defs    map[string]*SLO      // 已注册的 SLO 定义
	events  map[string][]*Event  // 各 SLO 的事件历史
	alerts  atomic.Uint64        // 累计告警次数
	reports atomic.Uint64        // Compute 累计调用次数
	now     func() time.Time     // 时间源（便于测试注入）
}

// NewTracker 构造一个空的 Tracker。
func NewTracker() *Tracker {
	return &Tracker{
		defs:   make(map[string]*SLO),
		events: make(map[string][]*Event),
		now:    time.Now,
	}
}

// WithTime 覆盖时间源（用于测试）。
//
// 返回 *Tracker 以支持链式调用：t := NewTracker().WithTime(fakeNow)。
func (t *Tracker) WithTime(now func() time.Time) *Tracker {
	t.now = now
	return t
}

// Register 添加一个 SLO 定义。
//
// 重名返回错误。
func (t *Tracker) Register(s *SLO) error {
	if err := s.Validate(); err != nil {
		return err
	}
	t.mu.Lock()
	if _, ok := t.defs[s.Name]; ok {
		t.mu.Unlock()
		return fmt.Errorf("slo %q already registered", s.Name)
	}
	t.defs[s.Name] = s
	t.mu.Unlock()
	return nil
}

// MustRegister 在出错时直接 panic。
func (t *Tracker) MustRegister(s *SLO) {
	if err := t.Register(s); err != nil {
		panic(err)
	}
}

// Record 为某个 SLO 存储一次样本。
//
// 内部会补充 sample.At = t.now()，然后调用 evictLocked 淘汰过期事件。
func (t *Tracker) Record(name string, sample Sample) {
	t.mu.Lock()
	defer t.mu.Unlock()
	sample.At = t.now()
	t.events[name] = append(t.events[name], &Event{
		Success: sample.Success, Took: sample.Took, At: sample.At,
	})
	t.evictLocked(name)
}

// Compute 返回某个 SLO 的 Status。
//
// 未注册的 SLO 返回 (zero, false)。
func (t *Tracker) Compute(name string) (Status, bool) {
	t.mu.Lock()
	def, ok := t.defs[name]
	if !ok {
		t.mu.Unlock()
		return Status{}, false
	}
	events := make([]*Event, len(t.events[name]))
	copy(events, t.events[name])
	t.mu.Unlock()
	now := t.now()
	windowStart := now.Add(-def.Window)
	var succ, fail int
	durs := make([]time.Duration, 0, len(events))
	for _, e := range events {
		if e.At.Before(windowStart) {
			continue
		}
		durs = append(durs, e.Took)
		if e.Success {
			succ++
		} else {
			fail++
		}
	}
	total := succ + fail
	ratio := 1.0
	if total > 0 {
		ratio = float64(succ) / float64(total)
	}
	budget := 1.0 - def.Target
	var rem int
	if budget > 0 && total > 0 {
		allowedFails := budget * float64(total)
		rem = int(allowedFails) - fail
		if rem < 0 {
			rem = 0
		}
	}
	t.reports.Add(1)
	if !ratioBoundedBy(ratio, def.Target) {
		t.alerts.Add(1)
	}
	return Status{
		Name: name, Target: def.Target,
		Window: def.Window.String(),
		Samples: total, Successes: succ, Failures: fail,
		Ratio: ratio, ErrorBudget: budget, Remaining: rem,
		Healthy: ratioBoundedBy(ratio, def.Target),
		P50: percentile(durs, 0.50),
		P95: percentile(durs, 0.95),
		P99: percentile(durs, 0.99),
	}, true
}

// Snapshot 返回所有 SLO 的状态（按 SLO 名字典序）。
func (t *Tracker) Snapshot() []Status {
	t.mu.Lock()
	names := make([]string, 0, len(t.defs))
	for n := range t.defs {
		names = append(names, n)
	}
	t.mu.Unlock()
	sort.Strings(names)
	out := make([]Status, 0, len(names))
	for _, n := range names {
		if s, ok := t.Compute(n); ok {
			out = append(out, s)
		}
	}
	return out
}

// Alerts 返回告警计数（当前未达标事件累计）。
func (t *Tracker) Alerts() uint64 { return t.alerts.Load() }

// Reports 返回 Compute 的累计调用次数。
func (t *Tracker) Reports() uint64 { return t.reports.Load() }

// evictLocked 淘汰指定 SLO 早于 windowStart 的事件。
//
// 必须在持有 t.mu 的状态下调用。
func (t *Tracker) evictLocked(name string) {
	events := t.events[name]
	if len(events) == 0 {
		return
	}
	def, ok := t.defs[name]
	if !ok {
		return
	}
	windowStart := t.now().Add(-def.Window)
	i := 0
	for ; i < len(events); i++ {
		if events[i].At.After(windowStart) || events[i].At.Equal(windowStart) {
			break
		}
	}
	if i > 0 {
		t.events[name] = append([]*Event{}, events[i:]...)
	}
}
