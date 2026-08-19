// Package latch 提供多种粒度的并发锁包装：
// - RWLatch：包装 sync.RWMutex 并暴露读者/写者计数
// - Striped：按 key 分片锁，降低热点
// - Simple：sync.Mutex 的别名包装
package latch

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
)

// RWLatch 是一个带计数信息的读写锁包装。
type RWLatch struct {
	mu       sync.RWMutex
	readers  atomic.Int32
	writers  atomic.Int32
}

// NewRWLatch 创建一个 RWLatch。
func NewRWLatch() *RWLatch { return &RWLatch{} }

// RLock 加读锁。
func (r *RWLatch) RLock() {
	r.mu.RLock()
	r.readers.Add(1)
}

// RUnlock 释放读锁。
func (r *RWLatch) RUnlock() {
	r.readers.Add(-1)
	r.mu.RUnlock()
}

// Lock 加写锁。
func (r *RWLatch) Lock() {
	r.mu.Lock()
	r.writers.Add(1)
}

// Unlock 释放写锁。
func (r *RWLatch) Unlock() {
	r.writers.Add(-1)
	r.mu.Unlock()
}

// Stats 返回当前计数。
func (r *RWLatch) Stats() RWLatchStats {
	return RWLatchStats{Readers: r.readers.Load(), Writers: r.writers.Load()}
}

// RWLatchStats 是 RWLatch 状态。
type RWLatchStats struct {
	Readers int32 `json:"readers"`
	Writers int32 `json:"writers"`
}

// Simple 是 sync.Mutex 的别名。
type Simple struct {
	mu sync.Mutex
}

// Lock 锁定。
func (s *Simple) Lock() { s.mu.Lock() }

// Unlock 释放。
func (s *Simple) Unlock() { s.mu.Unlock() }

// Striped 提供按 key 分片的锁集合，减少全局竞争。
type Striped struct {
	size  int
	locks []sync.Mutex
}

// NewStriped 创建一个 size 条 mutex 的 Striped。
func NewStriped(size int) *Striped {
	if size <= 0 {
		size = 16
	}
	return &Striped{size: size, locks: make([]sync.Mutex, size)}
}

// Lock 锁定 key 对应的分片。
func (s *Striped) Lock(key string) {
	s.locks[s.index(key)].Lock()
}

// Unlock 释放 key 对应的分片。
func (s *Striped) Unlock(key string) {
	s.locks[s.index(key)].Unlock()
}

// Do 在 key 对应的分片锁内执行 fn。
func (s *Striped) Do(key string, fn func()) {
	s.Lock(key)
	defer s.Unlock(key)
	fn()
}

func (s *Striped) index(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % s.size
}
