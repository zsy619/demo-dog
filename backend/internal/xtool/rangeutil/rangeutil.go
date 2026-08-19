// Package rangeutil 提供整数/字符串区间合并与覆盖判定等辅助。
package rangeutil

import "sort"

// Range 表示一个闭区间 [Start, End]。
type Range struct {
	Start int64
	End   int64
}

// Contains 返回 r 是否包含 x。
func (r Range) Contains(x int64) bool {
	return x >= r.Start && x <= r.End
}

// Overlaps 返回两个区间是否有交集。
func (r Range) Overlaps(o Range) bool {
	return r.Start <= o.End && o.Start <= r.End
}

// Merge 合并多个区间，返回互不相交的已合并区间。
func Merge(rs []Range) []Range {
	if len(rs) == 0 {
		return nil
	}
	sorted := make([]Range, len(rs))
	copy(sorted, rs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	out := []Range{sorted[0]}
	for _, r := range sorted[1:] {
		last := &out[len(out)-1]
		if r.Start <= last.End+1 {
			if r.End > last.End {
				last.End = r.End
			}
		} else {
			out = append(out, r)
		}
	}
	return out
}

// Subtract 从 r 中移除 o，返回剩余部分。
func Subtract(r, o Range) []Range {
	if !r.Overlaps(o) {
		return []Range{r}
	}
	out := []Range{}
	if o.Start > r.Start {
		out = append(out, Range{Start: r.Start, End: o.Start - 1})
	}
	if o.End < r.End {
		out = append(out, Range{Start: o.End + 1, End: r.End})
	}
	return out
}

// Length 返回区间长度（含端点）。
func (r Range) Length() int64 {
	return r.End - r.Start + 1
}
