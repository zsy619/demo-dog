// 包级 metrics 注册表。我们刻意保持只使用 stdlib
// (本后端没有任何外部依赖)，因此 metrics
// 基础设施是手写的：一个带 label 集的小型 histogram 加上
// Prometheus 文本导出器。其形态与 prom client 库输出保持一致，
// 因此任何开箱即用的 scraper 都能工作。

package api

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// labels 是用于 hash 和导出的标准 label-key 排序。
// 我们对 key 进行排序，使相同的 label 始终 hash 到相同的
// bucket，无论 map 迭代顺序如何。
func labels(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%s=%q", k, m[k])
	}
	return b.String()
}

// histogramSeries 是带固定 bucket
// 边界集的 per-label histogram。Bucket 为上界 (Prometheus 约定)。
type histogramSeries struct {
	mu      sync.Mutex
	labels  map[string]string
	bounds  []float64
	counts  []uint64   // len(bounds) + 1; last is +Inf overflow
	sum     float64
	count   uint64
}

func newHistogram(labels map[string]string, bounds []float64) *histogramSeries {
	h := &histogramSeries{
		labels: labels,
		bounds: append([]float64(nil), bounds...),
		counts: make([]uint64, len(bounds)+1),
	}
	return h
}

func (h *histogramSeries) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	idx := len(h.bounds)
	for i, b := range h.bounds {
		if v <= b {
			idx = i
			break
		}
	}
	h.counts[idx]++
	h.sum += v
	h.count++
}

func (h *histogramSeries) WriteText(w io.Writer, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	lbls := labels(h.labels)
	cum := uint64(0)
	for i, b := range h.bounds {
		cum += h.counts[i]
		fmt.Fprintf(w, "%s_bucket{%s,le=%q} %d\n", name, lbls, formatBound(b), cum)
	}
	cum += h.counts[len(h.bounds)]
	fmt.Fprintf(w, "%s_bucket{%s,le=%q} %d\n", name, lbls, infToken, cum)
	fmt.Fprintf(w, "%s_sum{%s} %g\n", name, lbls, h.sum)
	fmt.Fprintf(w, "%s_count{%s} %d\n", name, lbls, h.count)
}

const infToken = "+Inf"

func formatBound(b float64) string {
	return fmt.Sprintf("%g", b)
}

// histogramVec 是按 label tuple 的 hash 索引的带标签 histogram vector。
// 通过 map 实现 O(1) 查找。
type histogramVec struct {
	mu       sync.Mutex
	bounds   []float64
	children map[string]*histogramSeries
}

func newHistogramVec(bounds []float64) *histogramVec {
	return &histogramVec{
		bounds:   bounds,
		children: make(map[string]*histogramSeries),
	}
}

func (v *histogramVec) WithLabelValues(vals ...string) *histogramSeries {
	if len(vals)%2 != 0 {
		panic("histogramVec.WithLabelValues: odd number of args")
	}
	m := make(map[string]string, len(vals)/2)
	for i := 0; i < len(vals); i += 2 {
		m[vals[i]] = vals[i+1]
	}
	key := labels(m)
	v.mu.Lock()
	defer v.mu.Unlock()
	if h, ok := v.children[key]; ok {
		return h
	}
	h := newHistogram(m, v.bounds)
	v.children[key] = h
	return h
}

func (v *histogramVec) WriteText(w io.Writer, name string) {
	v.mu.Lock()
	children := make([]*histogramSeries, 0, len(v.children))
	for _, h := range v.children {
		children = append(children, h)
	}
	v.mu.Unlock()
	for _, h := range children {
		h.WriteText(w, name)
	}
}

// HTTP 请求延迟的默认桶边界，单位为秒。
var requestDurationBuckets = []float64{
	0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30,
}

// requestDuration 是每个处理器的延迟直方图。
var requestDuration = newHistogramVec(requestDurationBuckets)

// WriteMetrics 以 Prometheus 文本格式输出每个已注册的指标。
func WriteMetrics(w io.Writer) {
	fmt.Fprintln(w, "# HELP dog_request_duration_seconds Request duration by method and route.")
	fmt.Fprintln(w, "# TYPE dog_request_duration_seconds histogram")
	requestDuration.WriteText(w, "dog_request_duration_seconds")
}
