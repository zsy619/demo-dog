// Package metrics Counter、Gauge、Histogram 三类指标。
//
// 文件职责拆分:
//   - counter.go  CounterVec + CounterSeries
//   - gauge.go    GaugeVec + GaugeSeries
//   - histogram.go HistogramVec + HistogramSeries
package metrics

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
	mu        sync.RWMutex      // 保护 values
	name      string            // 指标名
	help      string            // 帮助文本
	labelKeys []string          // 标签键
	values    map[string]*counterSeries // label 元组 → 序列
}

// NewCounterVec 构造一个带标签的计数器。
func NewCounterVec(name, help string, labelKeys []string) *CounterVec {
	return &CounterVec{name: name, help: help, labelKeys: append([]string{}, labelKeys...), values: make(map[string]*counterSeries)}
}

// Name 返回指标名。
func (c *CounterVec) Name() string { return c.name }

// Help 返回帮助文本。
func (c *CounterVec) Help() string { return c.help }

// Type 返回指标类型字符串。
func (c *CounterVec) Type() string { return "counter" }

// LabelNames 返回标签键列表。
func (c *CounterVec) LabelNames() []string { return c.labelKeys }

// WithLabelValues 返回给定标签值对应的序列。
//
// 值的数量必须与 LabelNames 一致,否则返回错误。
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

// counterSeries 是内部实现。
type counterSeries struct {
	value  atomic.Uint64 // float64 位模式
	labels []string      // 标签值
}

// Add 增加 n。
//
// n < 0 直接忽略;并发安全通过 CAS 实现。
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
