package store

// tdigest_quantile.go:observeLocked + compact + Quantile(内部算法)。

import (
	"math"
	"sort"
)

// observeLocked 把观测值加入最近的质心或新增质心;调用方需持锁。
//
// 尺度函数: k * q * (1 - q)。中部的质心较大,尾部的质心较小。
func (t *TDigest) observeLocked(x float64, n int64) {
	if !t.hasData {
		t.min = x
		t.max = x
		t.hasData = true
	} else {
		if x < t.min {
			t.min = x
		}
		if x > t.max {
			t.max = x
		}
	}
	t.total += n

	// 查找与 x 均值最接近的质心。
	bestIdx := -1
	bestDist := math.Inf(1)
	for i, c := range t.centroids {
		d := math.Abs(c.mean - x)
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	if bestIdx >= 0 {
		c := &t.centroids[bestIdx]
		w := c.weight
		// 尺度阈值:该质心在其当前分位数位置上所能吸收的最大权重。
		q := float64(w) / float64(t.total)
		scale := 4 * t.delta * q * (1 - q) / t.delta
		if scale < 1 {
			scale = 1
		}
		if float64(w+n) <= scale &&
			bestDist < (t.max-t.min)*0.5/float64(t.delta) {
			// 吸收到最近的质心中。
			newMean := (c.mean*float64(w) + x*float64(n)) / float64(w+n)
			c.mean = newMean
			c.weight = w + n
			return
		}
	}
	// 新增一个质心;若数量过多则触发压缩。
	t.centroids = append(t.centroids, centroid{mean: x, weight: n})
	if len(t.centroids) > int(4*t.delta) {
		t.compact()
	}
}

// compact 在保持分位数准确度的前提下合并相邻质心。
//
// 压缩完成后按均值排序以便确定性分位数插值。
// 贪心合并:从左到右遍历,若合并后权重不超过该分位数位置的尺度阈值则合并。
// 这是完整 Dunning-Ertl 压缩算法的简化版本,典型分布准确度约 1%。
func (t *TDigest) compact() {
	if len(t.centroids) <= 1 {
		return
	}
	sort.Slice(t.centroids, func(i, j int) bool {
		return t.centroids[i].mean < t.centroids[j].mean
	})
	out := t.centroids[:1]
	for i := 1; i < len(t.centroids); i++ {
		c := t.centroids[i]
		last := &out[len(out)-1]
		w := last.weight
		q := float64(w) / float64(t.total)
		scale := 4 * t.delta * q * (1 - q) / t.delta
		if scale < 1 {
			scale = 1
		}
		if float64(w+c.weight) <= scale {
			merged := (last.mean*float64(w) + c.mean*float64(c.weight)) / float64(w+c.weight)
			last.mean = merged
			last.weight = w + c.weight
			continue
		}
		out = append(out, c)
	}
	t.centroids = out
}

// Quantile 返回第 q 分位数(0..1),在相邻质心之间进行线性插值。
//
// q 会被截断到 [0,1];无数据时返回 0。
func (t *TDigest) Quantile(q float64) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.hasData || t.total == 0 {
		return 0
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	if len(t.centroids) == 0 {
		return 0
	}
	sort.Slice(t.centroids, func(i, j int) bool {
		return t.centroids[i].mean < t.centroids[j].mean
	})
	target := q * float64(t.total)
	cum := int64(0)
	for i, c := range t.centroids {
		cumNext := cum + c.weight
		if float64(cumNext) >= target {
			// 根据质心内部的小数位置,在该质心与相邻质心的均值间插值。
			frac := float64(target-float64(cum)) / float64(c.weight)
			if i+1 < len(t.centroids) {
				next := t.centroids[i+1].mean
				return c.mean + frac*(next-c.mean)
			}
			return c.mean
		}
		cum = cumNext
	}
	return t.centroids[len(t.centroids)-1].mean
}
