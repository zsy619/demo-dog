// Package bptree 提供一个内存版的简化 B+ 树。
// 仅实现插入、查询、范围扫描，不实现删除/分裂合并。
// 主要用于教学与小型索引场景。
package bptree

import "sort"

// KV 是 B+ 树的一项。
type KV struct {
	K int64
	V any
}

// BPTree 是一个有序 KV 集合（按 K 排序）。
type BPTree struct {
	order int
	data  []KV
}

// New 创建阶数为 order 的 B+ 树（order >= 3）。
func New(order int) *BPTree {
	if order < 3 {
		order = 16
	}
	return &BPTree{order: order}
}

// Put 插入一个键值（K 已存在则覆盖）。
func (t *BPTree) Put(k int64, v any) {
	i := sort.Search(len(t.data), func(i int) bool { return t.data[i].K >= k })
	if i < len(t.data) && t.data[i].K == k {
		t.data[i].V = v
		return
	}
	t.data = append(t.data, KV{})
	copy(t.data[i+1:], t.data[i:])
	t.data[i] = KV{K: k, V: v}
}

// Get 读取键值。
func (t *BPTree) Get(k int64) (any, bool) {
	i := sort.Search(len(t.data), func(i int) bool { return t.data[i].K >= k })
	if i < len(t.data) && t.data[i].K == k {
		return t.data[i].V, true
	}
	return nil, false
}

// Range 返回 [lo, hi] 区间内的所有键值对（含端点）。
func (t *BPTree) Range(lo, hi int64) []KV {
	loI := sort.Search(len(t.data), func(i int) bool { return t.data[i].K >= lo })
	out := []KV{}
	for i := loI; i < len(t.data); i++ {
		if t.data[i].K > hi {
			break
		}
		out = append(out, t.data[i])
	}
	return out
}

// Len 返回元素数。
func (t *BPTree) Len() int { return len(t.data) }

// Order 返回阶数。
func (t *BPTree) Order() int { return t.order }

// Keys 按升序返回所有键。
func (t *BPTree) Keys() []int64 {
	out := make([]int64, len(t.data))
	for i, kv := range t.data {
		out[i] = kv.K
	}
	return out
}
