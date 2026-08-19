// Package idset 提供一个 int64 ID 集合（基于位图）。
package idset

import "sync"

// Set 是一个 int64 ID 的位图集合（稀疏友好）。
type Set struct {
	mu  sync.RWMutex
	min int64
	max int64
	m   map[int64]struct{}
}

// New 创建一个 ID Set。
func New() *Set { return &Set{m: make(map[int64]struct{})} }

// Add 添加 ID。
func (s *Set) Add(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = struct{}{}
	if len(s.m) == 1 {
		s.min = id
		s.max = id
		return
	}
	if id < s.min {
		s.min = id
	}
	if id > s.max {
		s.max = id
	}
}

// Remove 删除 ID。
func (s *Set) Remove(id int64) {
	s.mu.Lock()
	delete(s.m, id)
	s.mu.Unlock()
}

// Has 判断 ID 是否存在。
func (s *Set) Has(id int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[id]
	return ok
}

// Len 返回集合大小。
func (s *Set) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

// Min 返回最小 ID（无元素时返回 0, false）。
func (s *Set) Min() (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.m) == 0 {
		return 0, false
	}
	return s.min, true
}

// Max 返回最大 ID（无元素时返回 0, false）。
func (s *Set) Max() (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.m) == 0 {
		return 0, false
	}
	return s.max, true
}

// ToSlice 导出全部 ID（顺序随机）。
func (s *Set) ToSlice() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int64, 0, len(s.m))
	for id := range s.m {
		out = append(out, id)
	}
	return out
}

// Union 返回并集（与 other 的并集）。
func (s *Set) Union(other *Set) *Set {
	out := New()
	for _, id := range s.ToSlice() {
		out.Add(id)
	}
	for _, id := range other.ToSlice() {
		out.Add(id)
	}
	return out
}

// Clear 清空。
func (s *Set) Clear() {
	s.mu.Lock()
	s.m = make(map[int64]struct{})
	s.min = 0
	s.max = 0
	s.mu.Unlock()
}
