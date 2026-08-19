// Package metrics 提供轻量级的指标计数器：
// 计数、累计值、简单直方图（滑动窗口近似）。
package metrics

import "sync"

// Registry 收集指标的容器。
type Registry struct {
	mu       sync.RWMutex
	counters map[string]*Counter
	totals   map[string]*Total
	hist     map[string]*Hist
}

// New 创建一个 Registry。
func New() *Registry {
	return &Registry{
		counters: make(map[string]*Counter),
		totals:   make(map[string]*Total),
		hist:     make(map[string]*Hist),
	}
}

// Counter 是单调递增计数器。
type Counter struct {
	mu  sync.Mutex
	val uint64
}

// Add 增加 n。
func (c *Counter) Add(n uint64) {
	c.mu.Lock()
	c.val += n
	c.mu.Unlock()
}

// Value 返回当前值。
func (c *Counter) Value() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.val
}

// Counter 注册或获取一个 Counter。
func (r *Registry) Counter(name string) *Counter {
	r.mu.RLock()
	c, ok := r.counters[name]
	r.mu.RUnlock()
	if ok {
		return c
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok = r.counters[name]; ok {
		return c
	}
	c = &Counter{}
	r.counters[name] = c
	return c
}

// Total 是带标签的累计指标。
type Total struct {
	mu  sync.Mutex
	sum float64
	n   uint64
}

// Observe 添加一个观测值。
func (t *Total) Observe(v float64) {
	t.mu.Lock()
	t.sum += v
	t.n++
	t.mu.Unlock()
}

// Mean 返回平均值。
func (t *Total) Mean() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.n == 0 {
		return 0
	}
	return t.sum / float64(t.n)
}

// Sum 返回累计值。
func (t *Total) Sum() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sum
}

// Count 返回观测次数。
func (t *Total) Count() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.n
}

// Total 注册或获取一个 Total。
func (r *Registry) Total(name string) *Total {
	r.mu.RLock()
	t, ok := r.totals[name]
	r.mu.RUnlock()
	if ok {
		return t
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok = r.totals[name]; ok {
		return t
	}
	t = &Total{}
	r.totals[name] = t
	return t
}

// Hist 是一个简单的桶统计（固定边界）。
type Hist struct {
	mu    sync.Mutex
	count uint64
	sum   float64
	min   float64
	max   float64
}

// Observe 添加观测。
func (h *Hist) Observe(v float64) {
	h.mu.Lock()
	h.count++
	h.sum += v
	if h.count == 1 || v < h.min {
		h.min = v
	}
	if v > h.max {
		h.max = v
	}
	h.mu.Unlock()
}

// Snapshot 是 Hist 快照。
type Snapshot struct {
	Count uint64  `json:"count"`
	Sum   float64 `json:"sum"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Mean  float64 `json:"mean"`
}

// Snap 返回 Hist 快照。
func (h *Hist) Snap() Snapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count == 0 {
		return Snapshot{}
	}
	return Snapshot{
		Count: h.count,
		Sum:   h.sum,
		Min:   h.min,
		Max:   h.max,
		Mean:  h.sum / float64(h.count),
	}
}

// Hist 注册或获取 Hist。
func (r *Registry) Hist(name string) *Hist {
	r.mu.RLock()
	h, ok := r.hist[name]
	r.mu.RUnlock()
	if ok {
		return h
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok = r.hist[name]; ok {
		return h
	}
	h = &Hist{min: 0, max: 0}
	r.hist[name] = h
	return h
}
