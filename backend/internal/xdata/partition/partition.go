// Package partition 提供基于一致性哈希的简单分片。
package partition

import (
	"hash/fnv"
	"sort"
	"sync"
)

// Ring 是一致性哈希环。
type Ring struct {
	mu      sync.RWMutex
	nodes   map[uint64]string
	keys    []uint64
	replicas int
}

// New 创建一个默认 16 副本的 Ring。
func New() *Ring {
	return &Ring{replicas: 16, nodes: make(map[uint64]string)}
}

// SetReplicas 设置每个节点的副本数。
func (r *Ring) SetReplicas(n int) {
	if n < 1 {
		n = 1
	}
	r.replicas = n
}

// Add 添加一个节点。
func (r *Ring) Add(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i < r.replicas; i++ {
		h := hashKey(replicaKey(node, i))
		r.nodes[h] = node
		r.keys = append(r.keys, h)
	}
	sort.Slice(r.keys, func(i, j int) bool { return r.keys[i] < r.keys[j] })
}

// Remove 删除一个节点。
func (r *Ring) Remove(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	newKeys := r.keys[:0]
	for _, k := range r.keys {
		if r.nodes[k] != node {
			newKeys = append(newKeys, k)
		} else {
			delete(r.nodes, k)
		}
	}
	r.keys = newKeys
}

// Get 返回 key 对应的节点名。
func (r *Ring) Get(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.keys) == 0 {
		return ""
	}
	h := hashKey(key)
	idx := sort.Search(len(r.keys), func(i int) bool { return r.keys[i] >= h })
	if idx == len(r.keys) {
		idx = 0
	}
	return r.nodes[r.keys[idx]]
}

// Nodes 返回所有唯一节点。
func (r *Ring) Nodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	out := make([]string, 0, len(r.nodes))
	for _, n := range r.nodes {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// Len 返回唯一节点数。
func (r *Ring) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	for _, n := range r.nodes {
		seen[n] = true
	}
	return len(seen)
}

func hashKey(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func replicaKey(node string, i int) string {
	b := []byte(node)
	b = append(b, byte(i>>8), byte(i))
	return string(b)
}
