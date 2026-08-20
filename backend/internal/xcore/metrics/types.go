package metrics

// Counter、Gauge、Histogram 三类指标。

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// CounterVec 是一个带标签的计数器。
type CounterVec struct {
	mu        sync.RWMutex
	name      string
	help      string
	labelKeys []string
	values    map[string]*counterSeries
}

// NewCounterVec 构造一个带标签的计数器。
func NewCounterVec(name, help string, labelKeys []string) *CounterVec {
	return &CounterVec{name: name, help: help, labelKeys: append([]string{}, labelKeys...), values: make(map[string]*counterSeries)}
}

func (c *CounterVec) Name() string      { return c.name }
func (c *CounterVec) Help() string      { return c.help }
func (c *CounterVec) Type() string      { return "counter" }
func (c *CounterVec) LabelNames() []string { return c.labelKeys }

// WithLabelValues 返回给定标签值对应的序列。
// 当值的数量不匹配时返回错误。
func (c *CounterVec) WithLabelValues(values ...string) (*CounterSeries, error) {
	if len(values) != len(c.labelKeys) {
		return nil, fmt.Errorf("counter %q wants %d label values, got %d", c.name, len(c.labelKeys), len(values))
	}
	k := strings.Join(values, "\x00")
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.values[k]
	if !ok {
		s = &counterSeries{labels: append([]string{}, values...)}
		c.values[k] = s
	}
	return &CounterSeries{s}, nil
}

// CounterSeries 表示一条带标签的计数器序列。
type CounterSeries struct {
	*counterSeries
}

type counterSeries struct {
	value  atomic.Uint64 // bits of float64
	labels []string
}

// Add 增加 n。
func (s *CounterSeries) Add(n float64) {
	if n < 0 {
		return
	}
	for {
		old := s.value.Load()
		oldF := math.Float64frombits(old)
		newF := oldF + n
		if s.value.CompareAndSwap(old, math.Float64bits(newF)) {
			return
		}
	}
}

// Inc 增加 1。
func (s *CounterSeries) Inc() { s.Add(1) }

// Value 返回当前值。
func (s *CounterSeries) Value() float64 {
	return math.Float64frombits(s.value.Load())
}

// WriteText 输出计数器的文本行。
func (c *CounterVec) WriteText(w io.Writer) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.values))
	for k := range c.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := c.values[k]
		if _, err := io.WriteString(w, c.name); err != nil {
			return err
		}
		if len(c.labelKeys) > 0 {
			if err := WriteLabels(w, c.labelKeys, s.labels); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, " %g\n", math.Float64frombits(s.value.Load())); err != nil {
			return err
		}
	}
	return nil
}

// GaugeVec 是一个带标签的仪表盘。
type GaugeVec struct {
	mu        sync.RWMutex
	name      string
	help      string
	labelKeys []string
	values    map[string]*gaugeSeries
}

// NewGaugeVec 构造一个带标签的仪表盘。
func NewGaugeVec(name, help string, labelKeys []string) *GaugeVec {
	return &GaugeVec{name: name, help: help, labelKeys: append([]string{}, labelKeys...), values: make(map[string]*gaugeSeries)}
}

func (g *GaugeVec) Name() string         { return g.name }
func (g *GaugeVec) Help() string         { return g.help }
func (g *GaugeVec) Type() string         { return "gauge" }
func (g *GaugeVec) LabelNames() []string { return g.labelKeys }

// WithLabelValues 返回仪表盘序列。
func (g *GaugeVec) WithLabelValues(values ...string) (*GaugeSeries, error) {
	if len(values) != len(g.labelKeys) {
		return nil, fmt.Errorf("gauge %q wants %d label values, got %d", g.name, len(g.labelKeys), len(values))
	}
	k := strings.Join(values, "\x00")
	g.mu.Lock()
	defer g.mu.Unlock()
	s, ok := g.values[k]
	if !ok {
		s = &gaugeSeries{labels: append([]string{}, values...)}
		g.values[k] = s
	}
	return &GaugeSeries{s}, nil
}

// GaugeSeries 包装一条序列。
type GaugeSeries struct {
	*gaugeSeries
}

type gaugeSeries struct {
	bits   atomic.Uint64
	labels []string
}

// Set 替换当前值。
func (g *GaugeSeries) Set(v float64) {
	g.bits.Store(math.Float64bits(v))
}

// Add 自增。
func (g *GaugeSeries) Add(v float64) {
	for {
		old := g.bits.Load()
		newV := math.Float64frombits(old) + v
		if g.bits.CompareAndSwap(old, math.Float64bits(newV)) {
			return
		}
	}
}

// Inc adds 1.
func (g *GaugeSeries) Inc() { g.Add(1) }

// Dec 减去 1。
func (g *GaugeSeries) Dec() { g.Add(-1) }

// Value returns the current value.
func (g *GaugeSeries) Value() float64 {
	return math.Float64frombits(g.bits.Load())
}

// WriteText 输出仪表盘的文本行。
func (g *GaugeVec) WriteText(w io.Writer) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	keys := make([]string, 0, len(g.values))
	for k := range g.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := g.values[k]
		if _, err := io.WriteString(w, g.name); err != nil {
			return err
		}
		if len(g.labelKeys) > 0 {
			if err := WriteLabels(w, g.labelKeys, s.labels); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, " %g\n", math.Float64frombits(s.bits.Load())); err != nil {
			return err
		}
	}
	return nil
}
