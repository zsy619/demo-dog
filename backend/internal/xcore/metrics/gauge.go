package metrics

// gauge.go:GaugeVec + GaugeSeries。
//
// Gauge 表示可增可减的瞬时值(如内存使用、温度)。

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// GaugeVec 是一个带标签的仪表盘。
type GaugeVec struct {
	mu        sync.RWMutex    // 保护 values
	name      string          // 指标名
	help      string          // 帮助文本
	labelKeys []string        // 标签键
	values    map[string]*gaugeSeries // label 元组 → 序列
}

// NewGaugeVec 构造一个带标签的仪表盘。
func NewGaugeVec(name, help string, labelKeys []string) *GaugeVec {
	return &GaugeVec{name: name, help: help, labelKeys: append([]string{}, labelKeys...), values: make(map[string]*gaugeSeries)}
}

// Name 返回指标名。
func (g *GaugeVec) Name() string { return g.name }

// Help 返回帮助文本。
func (g *GaugeVec) Help() string { return g.help }

// Type 返回指标类型字符串。
func (g *GaugeVec) Type() string { return "gauge" }

// LabelNames 返回标签键列表。
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

// gaugeSeries 是内部实现。
type gaugeSeries struct {
	bits   atomic.Uint64 // float64 位模式
	labels []string      // 标签值
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

// Inc 增加 1。
func (g *GaugeSeries) Inc() { g.Add(1) }

// Dec 减去 1。
func (g *GaugeSeries) Dec() { g.Add(-1) }

// Value 返回当前值。
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
