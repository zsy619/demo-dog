// Package seq 提供单调递增的原子序号生成器。
package seq

import "sync/atomic"

// Seq 是 int64 原子序号。
type Seq struct {
	v atomic.Int64
}

// New 创建一个从 start 开始的序号生成器。
func New(start int64) *Seq {
	s := &Seq{}
	s.v.Store(start)
	return s
}

// Next 返回并递增当前值。
func (s *Seq) Next() int64 {
	return s.v.Add(1) - 1
}

// Peek 返回当前值（不递增）。
func (s *Seq) Peek() int64 {
	return s.v.Load()
}

// Reset 重置序号到 start。
func (s *Seq) Reset(start int64) {
	s.v.Store(start)
}

// SetCAS 在旧值等于 expected 时设置为 new。
func (s *Seq) SetCAS(expected, new int64) bool {
	return s.v.CompareAndSwap(expected, new)
}
