// Package shardkv 提供一个分片 KV 存储：
// 通过 key 的哈希分散到 N 个独立 map，并发性能优于单一锁。
package shardkv

import (
	"hash/fnv"
	"sync"
)

// Store 是分片 KV。
type Store struct {
	shards []*shard
	mask   uint32
}

type shard struct {
	mu    sync.RWMutex
	items map[string][]byte
}

// New 创建一个 n 个分片的存储。
func New(n int) *Store {
	if n <= 0 {
		n = 16
	}
	m := 1
	for m < n {
		m <<= 1
	}
	s := &Store{shards: make([]*shard, m), mask: uint32(m - 1)}
	for i := range s.shards {
		s.shards[i] = &shard{items: make(map[string][]byte)}
	}
	return s
}

func (s *Store) pick(k string) *shard {
	h := fnv.New32a()
	h.Write([]byte(k))
	return s.shards[h.Sum32()&s.mask]
}

// Put 写入键值。
func (s *Store) Put(k string, v []byte) {
	sh := s.pick(k)
	sh.mu.Lock()
	sh.items[k] = v
	sh.mu.Unlock()
}

// Get 读取键值。
func (s *Store) Get(k string) ([]byte, bool) {
	sh := s.pick(k)
	sh.mu.RLock()
	v, ok := sh.items[k]
	sh.mu.RUnlock()
	return v, ok
}

// Delete 移除一个键。
func (s *Store) Delete(k string) {
	sh := s.pick(k)
	sh.mu.Lock()
	delete(sh.items, k)
	sh.mu.Unlock()
}

// Len 返回所有分片的元素总数。
func (s *Store) Len() int {
	n := 0
	for _, sh := range s.shards {
		sh.mu.RLock()
		n += len(sh.items)
		sh.mu.RUnlock()
	}
	return n
}

// Shards 返回分片数。
func (s *Store) Shards() int { return len(s.shards) }
