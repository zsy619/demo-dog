package slo

// internal.go：私有辅助函数。

import (
	"sort"
	"time"
)

// ratioBoundedBy 判断 ratio 是否满足 target（即 ratio >= target）。
//
// 当 ratio 接近或超过 target 时认为 SLO 达标。
func ratioBoundedBy(ratio, target float64) bool { return ratio >= target }

// percentile 计算样本的 p 百分位延迟（p ∈ [0, 1]）。
//
// 空样本返回 0；不对输入排序做原地修改。
func percentile(durs []time.Duration, p float64) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	sorted := append([]time.Duration{}, durs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
