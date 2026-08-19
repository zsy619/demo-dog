// Package segmentx 提供区间段（半开区间 [lo, hi)）管理。
package segmentx

import (
	"sort"
	"sync"
)

// Segment 是一个 [lo, hi) 区间段。
type Segment struct {
	Lo int64
	Hi int64
}

// Set 是一个不重叠的区间段集合（按 lo 排序）。
type Set struct {
	mu       sync.RWMutex
	segments []Segment
}

// New 创建一个空 Set。
func New() *Set { return &Set{} }

// Add 加入一个区间段，与现有区间合并保持不重叠。
func (s *Set) Add(lo, hi int64) {
	if lo >= hi {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	newSeg := Segment{lo, hi}
	out := make([]Segment, 0, len(s.segments)+1)
	inserted := false
	for _, sg := range s.segments {
		if !inserted && newSeg.Lo < sg.Lo {
			// 合并 newSeg 与 sg（若相邻）
			if newSeg.Hi >= sg.Lo {
				if newSeg.Hi > sg.Hi {
					newSeg.Hi = newSeg.Hi
				} else {
					newSeg.Hi = sg.Hi
				}
			}
			out = append(out, newSeg)
			inserted = true
			// 继续处理 sg（可能要再次合并）
			if newSeg.Hi >= sg.Hi {
				continue
			}
			out = append(out, Segment{newSeg.Hi, sg.Hi})
			continue
		}
		// 合并相邻
		if len(out) > 0 && out[len(out)-1].Hi >= sg.Lo {
			last := &out[len(out)-1]
			if sg.Hi > last.Hi {
				last.Hi = sg.Hi
			}
			continue
		}
		out = append(out, sg)
	}
	if !inserted {
		if len(out) > 0 && out[len(out)-1].Hi >= newSeg.Lo {
			last := &out[len(out)-1]
			if newSeg.Hi > last.Hi {
				last.Hi = newSeg.Hi
			}
		} else {
			out = append(out, newSeg)
		}
	}
	s.segments = out
}

// Contains 判断 v 是否在任意区间内。
func (s *Set) Contains(v int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sg := range s.segments {
		if v >= sg.Lo && v < sg.Hi {
			return true
		}
	}
	return false
}

// All 返回所有区间段。
func (s *Set) All() []Segment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Segment, len(s.segments))
	copy(out, s.segments)
	return out
}

// Sort 强制按 lo 排序（合并过程中已保持有序）。
func (s *Set) Sort() {
	s.mu.Lock()
	sort.Slice(s.segments, func(i, j int) bool { return s.segments[i].Lo < s.segments[j].Lo })
	s.mu.Unlock()
}
