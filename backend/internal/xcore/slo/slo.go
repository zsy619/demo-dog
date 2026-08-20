// Package slo SLO 追踪：记录成功/失败计数与烧毁率。
package slo

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// SLO 表示一项服务等级目标。
type SLO struct {
	Name        string
	Description string
	Target      float64 // 0..1, e.g. 0.999 for 99.9%
	Window      time.Duration
}

// Validate 对 SLO 执行完整性检查。
func (s *SLO) Validate() error {
	if s.Name == "" {
		return errors.New("name required")
	}
	if s.Target <= 0 || s.Target > 1 {
		return errors.New("target must be in (0,1]")
	}
	if s.Window <= 0 {
		return errors.New("window must be positive")
	}
	return nil
}

// Sample 表示一次观测：
//   Success = true 表示请求成功。
//   Took   = 请求耗时。
type Sample struct {
	Success bool
	Took    time.Duration
	At      time.Time
}

// Tracker 按 SLO 聚合样本并产出 Status
//（当前成功率、剩余错误预算）。
type Tracker struct {
	mu       sync.Mutex
	defs     map[string]*SLO
	events   map[string][]*Event
	alerts   atomic.Uint64
	reports  atomic.Uint64
	now      func() time.Time
}

// Event 是 Sample 的存储形态。
type Event struct {
	Success bool
	Took    time.Duration
	At      time.Time
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
func (t *Tracker) WithTime(now func() time.Time) *Tracker {
	t.now = now
	return t
}

// Register 添加一个 SLO 定义。
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
func (t *Tracker) Record(name string, sample Sample) {
	t.mu.Lock()
	defer t.mu.Unlock()
	sample.At = t.now()
	t.events[name] = append(t.events[name], &Event{
		Success: sample.Success, Took: sample.Took, At: sample.At,
	})
	t.evictLocked(name)
}

// Status 表示当前 SLO 的状态。
type Status struct {
	Name       string        `json:"name"`
	Target     float64       `json:"target"`
	Window     string        `json:"window"`
	Samples    int           `json:"samples"`
	Successes  int           `json:"successes"`
	Failures   int           `json:"failures"`
	Ratio      float64       `json:"ratio"`
	ErrorBudget float64      `json:"error_budget"`
	Remaining  int           `json:"remaining_failures"`
	Healthy    bool          `json:"healthy"`
	P50        time.Duration `json:"p50"`
	P95        time.Duration `json:"p95"`
	P99        time.Duration `json:"p99"`
}

// Compute 返回某个 SLO 的 Status。
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

// Snapshot 返回所有 SLO 的状态。
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

// Alerts 返回告警计数。
func (t *Tracker) Alerts() uint64 { return t.alerts.Load() }

// Reports 返回 Compute 的累计调用次数。
func (t *Tracker) Reports() uint64 { return t.reports.Load() }

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

func ratioBoundedBy(ratio, target float64) bool { return ratio >= target }

func percentile(durs []time.Duration, p float64) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	sorted := append([]time.Duration{}, durs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
