package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// HistogramVec 是一个带标签的直方图。
type HistogramVec struct {
	mu        sync.RWMutex
	name      string
	help      string
	labelKeys []string
	buckets   []float64
	values    map[string]*histogramSeries
}

// DefaultBuckets 是 Prometheus 默认的桶边界。
var DefaultBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

// NewHistogramVec 构造一个带标签的直方图。
func NewHistogramVec(name, help string, labelKeys []string, buckets []float64) *HistogramVec {
	if len(buckets) == 0 {
		buckets = DefaultBuckets
	}
	sorted := append([]float64{}, buckets...)
	sort.Float64s(sorted)
	return &HistogramVec{
		name:      name,
		help:      help,
		labelKeys: append([]string{}, labelKeys...),
		buckets:   sorted,
		values:    make(map[string]*histogramSeries),
	}
}

func (h *HistogramVec) Name() string         { return h.name }
func (h *HistogramVec) Help() string         { return h.help }
func (h *HistogramVec) Type() string         { return "histogram" }
func (h *HistogramVec) LabelNames() []string { return h.labelKeys }

// WithLabelValues 返回直方图序列。
func (h *HistogramVec) WithLabelValues(values ...string) (*HistogramSeries, error) {
	if len(values) != len(h.labelKeys) {
		return nil, fmt.Errorf("histogram %q wants %d label values, got %d", h.name, len(h.labelKeys), len(values))
	}
	k := strings.Join(values, "\x00")
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.values[k]
	if !ok {
		s = &histogramSeries{
			labels:   append([]string{}, values...),
			bucketCt: make([]uint64, len(h.buckets)),
		}
		h.values[k] = s
	}
	return &HistogramSeries{s, h}, nil
}

// HistogramSeries 表示一条带标签的直方图序列。
type HistogramSeries struct {
	*histogramSeries
	vec *HistogramVec
}

type histogramSeries struct {
	labels   []string
	bucketCt []uint64 // count per bucket boundary
	count    uint64
	sum      float64
}

// Observe 记录一次观测值。
func (s *HistogramSeries) Observe(v float64) {
	s.histogramSeries.count++
	s.histogramSeries.sum += v
	for i, b := range s.vec.buckets {
		if v <= b {
			s.histogramSeries.bucketCt[i]++
		}
	}
}

// Count 返回观测次数。
func (s *HistogramSeries) Count() uint64 { return s.histogramSeries.count }

// Sum 返回观测值的总和。
func (s *HistogramSeries) Sum() float64 { return s.histogramSeries.sum }

// WriteText 输出直方图的文本行。
func (h *HistogramVec) WriteText(w io.Writer) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	keys := make([]string, 0, len(h.values))
	for k := range h.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := h.values[k]
		bucketCounts := s.bucketCt
		if bucketCounts == nil {
			bucketCounts = make([]uint64, len(h.buckets))
		}
		var cumulative uint64
		for i, b := range h.buckets {
			cumulative += bucketCounts[i]
			lvs := append([]string{}, s.labels...)
			le := fmt.Sprintf("%g", b)
			lvs = append(lvs, le)
			if _, err := io.WriteString(w, h.name+"_bucket"); err != nil {
				return err
			}
			if err := WriteLabels(w, append(append([]string{}, h.labelKeys...), "le"), lvs); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, " %d\n", cumulative); err != nil {
				return err
			}
		}
		// +Inf 桶 = 总样本数
		lvs := append([]string{}, s.labels...)
		lvs = append(lvs, "+Inf")
		if _, err := io.WriteString(w, h.name+"_bucket"); err != nil {
			return err
		}
		if err := WriteLabels(w, append(append([]string{}, h.labelKeys...), "le"), lvs); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, " %d\n", s.count); err != nil {
			return err
		}
		if _, err := io.WriteString(w, h.name+"_count"); err != nil {
			return err
		}
		if err := WriteLabels(w, h.labelKeys, s.labels); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, " %d\n", s.count); err != nil {
			return err
		}
		if _, err := io.WriteString(w, h.name+"_sum"); err != nil {
			return err
		}
		if err := WriteLabels(w, h.labelKeys, s.labels); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, " %g\n", s.sum); err != nil {
			return err
		}
	}
	return nil
}
