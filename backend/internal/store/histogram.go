package store

import (
	"sort"
	"sync"

	"github.com/zsy619/demo-dog/backend/internal/model"
)

// histogramAgg aggregates histogram data points for a single (service, name)
// series. It tracks:
//   * the latest explicit bucket boundaries + per-bucket counts (OTel format)
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

	// td is the streaming quantile estimator. Updated by ObserveRaw()
	// (callers feeding scalar metric points) and by add() when the
	// exporter provided a sum/n for an explicit-bucket histogram.
	td *TDigest
}

// newHistogramAgg builds a histogramAgg from a single OTel-style data
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
	// Derive total/count if exporter omitted it.
	if h.total == 0 {
		for _, c := range h.counts {
			h.total += c
		}
	}
	return h
}

// add merges another OTel histogram data point into the aggregate.
func (h *histogramAgg) add(p model.MetricPoint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Bounds mismatch -> rebuild from scratch. Cheap enough since this
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
	// Also feed the t-digest with the bucket midpoints so quantile
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

// ObserveRaw feeds a single scalar observation into the t-digest.
// Used by non-histogram metric streams that need quantile answers.
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

// QuantileStreaming returns the q-th quantile (0..1) computed from
// the t-digest of raw observations. Use this when the upstream
// exporter does not supply explicit bucket bounds (scalar metric
// streams). Returns 0 if no data.
func (h *histogramAgg) QuantileStreaming(q float64) float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.td == nil {
		return 0
	}
	return h.td.Quantile(q)
}

// snapshot returns a copy of the current aggregate state for safe
// external consumption. Returns nil if no data has been added.
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

// quantile returns the q-th percentile (0..1) using the OTel
// histogram bucket boundaries. We treat each bucket as [lower, upper)
// with linear interpolation inside the bucket for finer granularity.
// Returns 0 if there is no data. q is clamped to [0,1].
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
			// Found the bucket containing the q-th observation.
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

// sameBounds compares two slices for bucket-bound equivalence. We
// require length match + element equality. Exporters that use a
// different number of buckets will trigger a reset, which is the
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

// sortBucketBounds returns a copy of bounds with the +Inf (math.MaxFloat64)
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
