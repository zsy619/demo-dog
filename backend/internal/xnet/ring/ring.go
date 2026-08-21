// Package ring 一致性哈希环：节点映射 + 副本定位。
package ring

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
	"sync"
)

// Ring 是一个带有虚拟节点的一致性哈希环。
type Ring struct {
	mu       sync.RWMutex
	replicas int
	keys     []uint32 // sorted
	hashmap  map[uint32]string
}

// New constructs a Ring with the given 副本 count (virtual
// nodes per node).
func New(replicas int) *Ring {
	if replicas <= 0 {
		replicas = 100
	}
	return &Ring{
		replicas: replicas,
		hashmap:  make(map[uint32]string),
	}
}

// Add 向环中引入一个节点。
func (r *Ring) Add(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i < r.replicas; i++ {
		h := hashKey(replicaKey(node, i))
		r.keys = append(r.keys, h)
		r.hashmap[h] = node
	}
	sort.Slice(r.keys, func(i, j int) bool { return r.keys[i] < r.keys[j] })
}

// Remove 从环中移除一个节点。
func (r *Ring) Remove(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.keys[:0]
	for _, k := range r.keys {
		if r.hashmap[k] != node {
			out = append(out, k)
		}
	}
	r.keys = out
	for k, v := range r.hashmap {
		if v == node {
			delete(r.hashmap, k)
		}
	}
}

// Lookup 返回负责 key 的节点。
func (r *Ring) Lookup(key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.keys) == 0 {
		return "", errors.New("ring is empty")
	}
	h := hashKey(key)
	idx := sort.Search(len(r.keys), func(i int) bool {
		return r.keys[i] >= h
	})
	if idx == len(r.keys) {
		idx = 0
	}
	return r.hashmap[r.keys[idx]], nil
}

// LookupN 返回 N distinct nodes responsible for key (with
// replicas on different physical machines if any).
func (r *Ring) LookupN(key string, n int) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.keys) == 0 {
		return nil, errors.New("ring is empty")
	}
	if n <= 0 || n > len(r.hashmap) {
		n = len(r.hashmap)
	}
	h := hashKey(key)
	idx := sort.Search(len(r.keys), func(i int) bool {
		return r.keys[i] >= h
	})
	out := make([]string, 0, n)
	seen := make(map[string]struct{})
	for i := 0; len(out) < n && i < len(r.keys); i++ {
		pos := (idx + i) % len(r.keys)
		node := r.hashmap[r.keys[pos]]
		if _, dup := seen[node]; dup {
			continue
		}
		seen[node] = struct{}{}
		out = append(out, node)
	}
	return out, nil
}

// Nodes 返回环中节点的唯一集合。
func (r *Ring) Nodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, n := range r.hashmap {
		seen[n] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Size 返回唯一节点的数量。
func (r *Ring) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, n := range r.hashmap {
		seen[n] = struct{}{}
	}
	return len(seen)
}

// Distribution 返回映射到每个节点的 key 的占比。可用于验证均衡性。
// node. Useful for verifying balance.
func (r *Ring) Distribution(keys []string) map[string]float64 {
	if len(keys) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, k := range keys {
		n, _ := r.Lookup(k)
		counts[n]++
	}
	out := make(map[string]float64, len(counts))
	for n, c := range counts {
		out[n] = float64(c) / float64(len(keys))
	}
	return out
}

func hashKey(s string) uint32 {
	h := sha256.Sum256([]byte(s))
	return binary.BigEndian.Uint32(h[:4])
}

func replicaKey(node string, i int) string {
	b := make([]byte, 0, len(node)+8)
	b = append(b, node...)
	b = append(b, byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
	return string(b)
}
