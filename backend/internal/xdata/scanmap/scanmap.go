// Package scanmap 提供带游标的 map 扫描（用于分页遍历等场景）。
package scanmap

import (
	"sort"
)

// Scanner 是一个按 key 排序的 map 扫描器。
type Scanner[V any] struct {
	keys   []string
	values map[string]V
	idx    int
}

// New 创建扫描器（kvs 是初始数据）。
func New[V any](kvs map[string]V) *Scanner[V] {
	s := &Scanner[V]{values: make(map[string]V, len(kvs))}
	for k, v := range kvs {
		s.keys = append(s.keys, k)
		s.values[k] = v
	}
	sort.Strings(s.keys)
	return s
}

// Next 返回下一对 (k, v)。返回 false 表示已遍历完。
func (s *Scanner[V]) Next() (string, V, bool) {
	if s.idx >= len(s.keys) {
		var zero V
		return "", zero, false
	}
	k := s.keys[s.idx]
	s.idx++
	return k, s.values[k], true
}

// Reset 重置游标。
func (s *Scanner[V]) Reset() { s.idx = 0 }

// Len 返回元素数。
func (s *Scanner[V]) Len() int { return len(s.keys) }

// Pos 返回当前游标位置。
func (s *Scanner[V]) Pos() int { return s.idx }

// Seek 跳到第一个 >= target 的位置。
func (s *Scanner[V]) Seek(target string) {
	s.idx = sort.Search(len(s.keys), func(i int) bool { return s.keys[i] >= target })
}

// Slice 返回全部键的有序副本。
func (s *Scanner[V]) Slice() []string {
	out := make([]string, len(s.keys))
	copy(out, s.keys)
	return out
}
