// Package metricsx 提供一个轻量的指标采集器：
// 支持 Counter / Gauge / Histogram，并提供按标签聚合。
package metricsx

import (
	"sort"
	"sync"
	"sync/atomic"
)

// Counter 是一个单调递增计数器。
type Counter struct {
	val atomic.Uint64
}

// Inc 增加 1。
func (c *Counter) Inc() { c.val.Add(1) }

// Add 增加 n。
func (c *Counter) Add(n uint64) { c.val.Add(n) }

// Value 返回当前值。
func (c *Counter) Value() uint64 { return c.val.Load() }

// Gauge 是一个可上下浮动的数。
type Gauge struct {
	bits atomic.Uint64
}

// Set 设置当前值。
func (g *Gauge) Set(v float64) {
	g.bits.Store(floatToBits(v))
}

// Value 返回当前值。
func (g *Gauge) Value() float64 {
	return bitsToFloat(g.bits.Load())
}

// Registry 持有命名指标。
type Registry struct {
	mu        sync.RWMutex
	counters  map[string]*Counter
	gauges    map[string]*Gauge
	histograms map[string]*Histogram
}

// NewRegistry 创建一个空 Registry。
func NewRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

// Counter 取（或创建）一个 Counter。
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

// Gauge 取（或创建）一个 Gauge。
func (r *Registry) Gauge(name string) *Gauge {
	r.mu.RLock()
	g, ok := r.gauges[name]
	r.mu.RUnlock()
	if ok {
		return g
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok = r.gauges[name]; ok {
		return g
	}
	g = &Gauge{}
	r.gauges[name] = g
	return g
}

// Histogram 取（或创建）一个 Histogram。
func (r *Registry) Histogram(name string, buckets []float64) *Histogram {
	r.mu.RLock()
	h, ok := r.histograms[name]
	r.mu.RUnlock()
	if ok {
		return h
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok = r.histograms[name]; ok {
		return h
	}
	h = newHistogram(buckets)
	r.histograms[name] = h
	return h
}

// Snapshot 是所有指标的导出视图。
type Snapshot struct {
	Counters   map[string]uint64  `json:"counters"`
	Gauges     map[string]float64 `json:"gauges"`
	Histograms map[string]HStats   `json:"histograms"`
}

// Snapshot 返回当前所有指标的快照。
func (r *Registry) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := Snapshot{
		Counters:   make(map[string]uint64, len(r.counters)),
		Gauges:     make(map[string]float64, len(r.gauges)),
		Histograms: make(map[string]HStats, len(r.histograms)),
	}
	for k, v := range r.counters {
		out.Counters[k] = v.Value()
	}
	for k, v := range r.gauges {
		out.Gauges[k] = v.Value()
	}
	for k, v := range r.histograms {
		out.Histograms[k] = v.Stats()
	}
	return out
}

// Names 按字典序返回所有指标名。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.counters)+len(r.gauges)+len(r.histograms))
	for k := range r.counters {
		out = append(out, "counter:"+k)
	}
	for k := range r.gauges {
		out = append(out, "gauge:"+k)
	}
	for k := range r.histograms {
		out = append(out, "hist:"+k)
	}
	sort.Strings(out)
	return out
}

// Reset 重置所有指标。
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters = make(map[string]*Counter)
	r.gauges = make(map[string]*Gauge)
	r.histograms = make(map[string]*Histogram)
}
