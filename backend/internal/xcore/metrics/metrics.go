// Package metrics Prometheus 指标注册器：Counter / Gauge / Histogram。
package metrics

// Prometheus-compatible metrics registry.
//
// Implements just enough of the Prometheus exposition format
// to ship a working /metrics endpoint without third-party deps.
// Supports Counter / Gauge / Histogram with multi-tenant
// labels.
//
// The exposition format follows the OpenMetrics text format:
//
//   # HELP <name> <description>
//   # TYPE <name> counter|gauge|histogram
//   <name>{label="value"} 7
//
// Histograms emit the standard _bucket / _count / _sum series.

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// Metric is the contract every metric type implements.
type Metric interface {
	Name() string
	Help() string
	Type() string
	WriteText(w io.Writer) error
	LabelNames() []string
}

// Registry owns the metric table.
type Registry struct {
	mu      sync.RWMutex
	metrics map[string]Metric
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{metrics: make(map[string]Metric)}
}

// Register adds a metric; returns error on name conflict or
// invalid name.
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

// MustRegister panics on duplicate / invalid name.
func (r *Registry) MustRegister(m Metric) {
	if err := r.Register(m); err != nil {
		panic(err)
	}
}

// Get returns a registered metric by name.
func (r *Registry) Get(name string) (Metric, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.metrics[name]
	return m, ok
}

// Names returns the registered metric names sorted.
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

// WriteText writes the Prometheus text exposition format.
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

// WriteLabels formats a label set for the text format.
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
