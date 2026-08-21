package store

import (
	"sort"
	"sync"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// histogramAgg 聚合单个 (service, name) 的直方图数据点
// 序列. It tracks:
//   * 最近的 explicit bucket boundaries + per-bucket counts (OTel format)
//   * a t-digest of raw scalar observations (Round 30) so quantile
//     answers are available even when exporters send scalar streams
//     without explicit buckets
//   * running totals
type histogramAgg struct {
	mu sync.Mutex

	bounds  []float64
	counts  []int64
	total   int64
	sum     float64
	min     float64
	max     float64
	hasData bool

	// td 是流式分位数估计器。由 ObserveRaw() 更新
	// (callers feeding scalar metric points) and by add() when the
	// exporter provided a sum/n for an explicit-bucket 直方图.
	td *TDigest
}

// newHistogramAgg 从单个 OTel 风格的数据
// point. The bucket bounds are taken from the point (assumed ascending,
// last entry is +Inf marker). bucket counts from the same point seed
// the aggregate; subsequent points with the same bounds add into the
// existing buckets. If the bounds change between calls we rebuild the
// aggregate (rare; means the server reconfigured).
func newHistogramAgg(p model.MetricPoint) *histogramAgg {
	h := &histogramAgg{
		bounds: append([]float64(nil), p.BucketBounds...),
		counts: append([]int64(nil), p.BucketCounts...),
		total:  p.HistogramCount,
		sum:    p.HistogramSum,
		min:    p.HistogramMin,
		max:    p.HistogramMax,
		td:     NewTDigest(100),
	}
	if !h.hasData {
		h.min = p.HistogramMin
		h.max = p.HistogramMax
		h.hasData = true
	}
	// 在导出端省略时推导 total/count。
	if h.total == 0 {
		for _, c := range h.counts {
			h.total += c
		}
	}
	return h
}

// add 将另一个 OTel 直方图数据点合并到聚合中。
func (h *histogramAgg) add(p model.MetricPoint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// 边界不匹配 -> 从头重建。此操作足够廉价，
	// happens only when the upstream reconfigured.
	if !sameBounds(h.bounds, p.BucketBounds) {
		h.bounds = append([]float64(nil), p.BucketBounds...)
		h.counts = append([]int64(nil), p.BucketCounts...)
		h.total = p.HistogramCount
		h.sum = p.HistogramSum
		h.min = p.HistogramMin
		h.max = p.HistogramMax
		h.hasData = true
		return
	}
	// 同时将桶中点送入 t-digest，以便分位数
	// answers remain accurate when the OTel bucket bounds shift
	// between snapshots. We approximate each bucket by its midpoint.
	if len(p.BucketCounts) == len(h.bounds) {
		for i, c := range p.BucketCounts {
			if c > 0 {
				lower := 0.0
				if i > 0 {
					lower = h.bounds[i-1]
				}
				upper := h.bounds[i]
				mid := (lower + upper) / 2
				h.td.ObserveBatch(mid, c)
			}
		}
	}
	for i, c := range p.BucketCounts {
		if i < len(h.counts) {
			h.counts[i] += c
		}
	}
	h.total += p.HistogramCount
	h.sum += p.HistogramSum
	if !h.hasData || p.HistogramMin < h.min {
		h.min = p.HistogramMin
	}
	if !h.hasData || p.HistogramMax > h.max {
		h.max = p.HistogramMax
	}
	h.hasData = true
}

// ObserveRaw 将单个标量观测值送入 t-digest。
// 由...使用 non-histogram metric streams that need quantile answers.
func (h *histogramAgg) ObserveRaw(x float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.td == nil {
		h.td = NewTDigest(100)
	}
	h.td.Observe(x)
	h.hasData = true
	if !h.hasData || x < h.min { h.min = x }
	if !h.hasData || x > h.max { h.max = x }
	h.sum += x
	h.total++
}

// QuantileStreaming 返回从计算的 q 阶分位数（0..1）
// the t-digest of raw observations. Use this when the upstream
// exporter does not supply explicit bucket bounds (scalar metric
// streams). 返回 0 if no data.
func (h *histogramAgg) QuantileStreaming(q float64) float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.td == nil {
		return 0
	}
	return h.td.Quantile(q)
}

// snapshot 返回当前聚合状态的副本，以便安全地
// external consumption. 返回 nil if no data has been added.
func (h *histogramAgg) snapshot() *model.HistogramView {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.hasData {
		return nil
	}
	bounds := append([]float64(nil), h.bounds...)
	counts := append([]int64(nil), h.counts...)
	return &model.HistogramView{
		Bounds: bounds,
		Counts: counts,
		Total:  h.total,
		Sum:    h.sum,
		Min:    h.min,
		Max:    h.max,
	}
}

// quantile 使用 OTel 方式返回第 q 百分位（0..1）
// 直方图 bucket boundaries. We treat each bucket as [lower, upper)
// with linear interpolation inside the bucket for finer granularity.
// 返回 0 if there is no data. q is clamped to [0,1].
func (h *histogramAgg) quantile(q float64) float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.hasData || h.total == 0 {
		return 0
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	target := float64(h.total) * q
	cum := int64(0)
	for i, c := range h.counts {
		cum += c
		if float64(cum) >= target {
			// 找到包含第 q 个观测值的桶。
			// Interpolate within the bucket for better precision.
			lower := 0.0
			if i > 0 {
				lower = h.bounds[i-1]
			}
			upper := h.bounds[i]
			bucketPos := float64(cum) - target
			if c > 0 {
				frac := bucketPos / float64(c)
				return upper - frac*(upper-lower)
			}
			return upper
		}
	}
	return h.max
}

// sameBounds 比较两个切片是否桶边界等价。我们
// require length match + element equality. Exporters that use a
// different 数量 buckets will trigger a reset, which is the
// safest behavior.
func sameBounds(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sortBucketBounds 返回 bounds 的副本，并将 +Inf（math.MaxFloat64）
// marker moved to the end if it is not already there. Some exporters put
// the overflow first; this normalizes so downstream code is uniform.
func sortBucketBounds(b []float64) []float64 {
	if len(b) == 0 {
		return b
	}
	out := append([]float64(nil), b...)
	sort.Float64s(out)
	return out
}
