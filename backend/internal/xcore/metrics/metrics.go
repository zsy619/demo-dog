// Package metrics Prometheus 指标注册器：Counter / Gauge / Histogram。
package metrics

// Prometheus 兼容的指标注册器。
//
// 仅实现 Prometheus 文本暴露格式的关键部分，
// 即可在不使用第三方依赖的前提下提供可用的 /metrics 端点。
// 支持 Counter / Gauge / Histogram，并支持多租户标签。
//
// 暴露格式遵循 OpenMetrics 文本格式：
//
//   # HELP <name> <description>
//   # TYPE <name> counter|gauge|histogram
//   <name>{label="value"} 7
//
// 直方图会输出标准的 _bucket / _count / _sum 三类序列。

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// Metric 是每种指标类型都必须实现的契约。
type Metric interface {
	Name() string
	Help() string
	Type() string
	WriteText(w io.Writer) error
	LabelNames() []string
}

// Registry 持有指标表。
type Registry struct {
	mu      sync.RWMutex
	metrics map[string]Metric
}

// NewRegistry 返回一个空的注册器。
func NewRegistry() *Registry {
	return &Registry{metrics: make(map[string]Metric)}
}

// Register 添加一个指标；若名称冲突或非法则返回错误。
func (r *Registry) Register(m Metric) error {
	if !validName(m.Name()) {
		return fmt.Errorf("invalid metric name %q", m.Name())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.metrics[m.Name()]; ok {
		return fmt.Errorf("metric %q already registered", m.Name())
	}
	r.metrics[m.Name()] = m
	return nil
}

// MustRegister 在重名或名称非法时直接 panic。
func (r *Registry) MustRegister(m Metric) {
	if err := r.Register(m); err != nil {
		panic(err)
	}
}

// Get 按名称返回一个已注册的指标。
func (r *Registry) Get(name string) (Metric, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.metrics[name]
	return m, ok
}

// Names 返回已注册指标名称（已排序）。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.metrics))
	for n := range r.metrics {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// WriteText 按 Prometheus 文本暴露格式输出。
func (r *Registry) WriteText(w io.Writer) error {
	r.mu.RLock()
	metrics := make([]Metric, 0, len(r.metrics))
	for _, m := range r.metrics {
		metrics = append(metrics, m)
	}
	r.mu.RUnlock()
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].Name() < metrics[j].Name() })
	for _, m := range metrics {
		if _, err := fmt.Fprintf(w, "# HELP %s %s\n", m.Name(), escapeHelp(m.Help())); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "# TYPE %s %s\n", m.Name(), m.Type()); err != nil {
			return err
		}
		if err := m.WriteText(w); err != nil {
			return err
		}
	}
	return nil
}

func validName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c == ':' && i > 0:
		case c == '_':
		default:
			if i == 0 {
				return false
			}
			return false
		}
	}
	return true
}

func escapeHelp(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// WriteLabels 为文本格式格式化一组标签。
func WriteLabels(w io.Writer, names []string, values []string) error {
	if len(names) == 0 {
		return nil
	}
	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	for i := range names {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		v := values[i]
		v = strings.ReplaceAll(v, "\\", "\\\\")
		v = strings.ReplaceAll(v, "\"", "\\\"")
		v = strings.ReplaceAll(v, "\n", "\\n")
		if _, err := fmt.Fprintf(w, "%s=%q", names[i], v); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "}")
	return err
}
