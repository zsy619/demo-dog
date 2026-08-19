// Package metricsx 的直方图实现。
package metricsx

import (
	"math"
	"sort"
	"sync"
)

// Histogram 是一个桶式直方图。
type Histogram struct {
	mu      sync.Mutex
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

func newHistogram(buckets []float64) *Histogram {
	cp := make([]float64, len(buckets))
	copy(cp, buckets)
	sort.Float64s(cp)
	return &Histogram{buckets: cp, counts: make([]uint64, len(cp)+1)}
}

// Observe 记录一次观察值。
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sum += v
	h.count++
	idx := sort.SearchFloat64s(h.buckets, v)
	h.counts[idx]++
}

// HStats 是直方图统计视图。
type HStats struct {
	Count   uint64    `json:"count"`
	Sum     float64   `json:"sum"`
	Min     float64   `json:"min"`
	Max     float64   `json:"max"`
	Avg     float64   `json:"avg"`
	Buckets []float64 `json:"buckets"`
	Counts  []uint64  `json:"counts"`
}

// Stats 返回当前统计。
func (h *Histogram) Stats() HStats {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count == 0 {
		return HStats{Buckets: append([]float64(nil), h.buckets...),
			Counts: append([]uint64(nil), h.counts...)}
	}
	avg := h.sum / float64(h.count)
	// Min / Max 从 count 中无法还原；用 -inf / +inf 表示未设置
	return HStats{
		Count:   h.count,
		Sum:     h.sum,
		Min:     math.Inf(-1),
		Max:     math.Inf(+1),
		Avg:     avg,
		Buckets: append([]float64(nil), h.buckets...),
		Counts:  append([]uint64(nil), h.counts...),
	}
}

// Reset 重置直方图。
func (h *Histogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count = 0
	h.sum = 0
	for i := range h.counts {
		h.counts[i] = 0
	}
}
