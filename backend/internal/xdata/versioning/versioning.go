// Package versioning 提供基于单调递增版本号的乐观锁：
// CAS 风格的 CompareAndSwap 与版本生成。
package versioning

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ErrStale 在版本号过旧时返回。
var ErrStale = errors.New("versioning: 版本过期")

// Version 是简单的 64 位版本号。
type Version struct {
	v atomic.Uint64
}

// New 创建一个从 1 开始的版本号。
func New() *Version { v := &Version{}; v.v.Store(1); return v }

// Current 返回当前版本。
func (v *Version) Current() uint64 { return v.v.Load() }

// Bump 原子递增并返回新值。
func (v *Version) Bump() uint64 { return v.v.Add(1) }

// CompareAndSwap 在 old 与当前相等时设置 new。
func (v *Version) CompareAndSwap(old, new uint64) bool {
	return v.v.CompareAndSwap(old, new)
}

// Guard 包装一个值并跟踪版本号。
type Guard[T any] struct {
	mu sync.Mutex
	v  T
	ver uint64
}

// NewGuard 创建一个初始值为 v 的 Guard。
func NewGuard[T any](v T) *Guard[T] {
	return &Guard[T]{v: v, ver: 1}
}

// Get 读取当前值与版本。
func (g *Guard[T]) Get() (T, uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.v, g.ver
}

// Update 基于期望版本写入新值；版本不匹配返回 ErrStale。
func (g *Guard[T]) Update(expected uint64, fn func(T) T) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ver != expected {
		return ErrStale
	}
	g.v = fn(g.v)
	g.ver++
	return nil
}

// Force 强制写入并提升版本。
func (g *Guard[T]) Force(fn func(T) T) {
	g.mu.Lock()
	g.v = fn(g.v)
	g.ver++
	g.mu.Unlock()
}
