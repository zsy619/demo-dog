// Package merkle 默克尔树：构建内容哈希摘要以校验一致性。
package merkle

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
)

// Tree 是对一组有序 keys 构建的二叉默克尔树。
type Tree struct {
	mu    sync.RWMutex
	keys  []string
	leaves [][]byte
	root  []byte
}

// New 由 keys 构造一棵默克尔树。
func New(keys []string) *Tree {
	sorted := append([]string{}, keys...)
	sort.Strings(sorted)
	t := &Tree{keys: sorted}
	t.leaves = make([][]byte, len(sorted))
	for i, k := range sorted {
		t.leaves[i] = leafHash(k)
	}
	t.root = buildRoot(t.leaves)
	return t
}

// Root 以十六进制字符串返回根哈希。
func (t *Tree) Root() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return hex.EncodeToString(t.root)
}

// Diff 返回 b 中不在 t 中的 key（或具有不同
// different value).
func (t *Tree) Diff(other *Tree) []string {
	t.mu.RLock()
	local := make(map[string]struct{}, len(t.keys))
	for _, k := range t.keys {
		local[k] = struct{}{}
	}
	t.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	out := make([]string, 0)
	for _, k := range other.keys {
		if _, ok := local[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Keys 返回排序后的 key 列表。
func (t *Tree) Keys() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]string, len(t.keys))
	copy(out, t.keys)
	return out
}

// Equal 当两棵树根相同时返回 true。
func (t *Tree) Equal(other *Tree) bool {
	if t == nil || other == nil {
		return t == other
	}
	return t.Root() == other.Root()
}

func leafHash(key string) []byte {
	h := sha256.New()
	h.Write([]byte("L"))
	h.Write([]byte(key))
	return h.Sum(nil)
}

func nodeHash(l, r []byte) []byte {
	h := sha256.New()
	h.Write([]byte("N"))
	h.Write(l)
	h.Write(r)
	return h.Sum(nil)
}

func buildRoot(leaves [][]byte) []byte {
	switch len(leaves) {
	case 0:
		return leafHash("")
	case 1:
		return leaves[0]
	}
	level := leaves
	for len(level) > 1 {
		next := make([][]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			if i+1 == len(level) {
				next = append(next, level[i])
			} else {
				next = append(next, nodeHash(level[i], level[i+1]))
			}
		}
		level = next
	}
	return level[0]
}

// Proof 是一个 key 的成员性证明。
type Proof struct {
	Key  string
	Path []string
}

// Proof 若 key 存在则返回其成员性证明。
// Returns nil if the key is not in the tree.
func (t *Tree) Proof(key string) *Proof {
	t.mu.RLock()
	defer t.mu.RUnlock()
	idx := sort.Search(len(t.keys), func(i int) bool {
		return t.keys[i] >= key
	})
	if idx >= len(t.keys) || t.keys[idx] != key {
		return nil
	}
	path := make([]string, 0)
	level := append([][]byte{}, t.leaves...)
	pos := idx
	for len(level) > 1 {
		var sibling []byte
		if pos%2 == 0 {
			if pos+1 < len(level) {
				sibling = level[pos+1]
			} else {
				sibling = level[pos]
			}
		} else {
			sibling = level[pos-1]
		}
		path = append(path, hex.EncodeToString(sibling))
		next := make([][]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			if i+1 == len(level) {
				next = append(next, level[i])
			} else {
				next = append(next, nodeHash(level[i], level[i+1]))
			}
		}
		level = next
		pos /= 2
	}
	return &Proof{Key: key, Path: path}
}

// VerifyProof 检查证明是否与树根匹配。
func (t *Tree) VerifyProof(p *Proof) bool {
	leaf := leafHash(p.Key)
	cur := leaf
	idx := sort.Search(len(t.keys), func(i int) bool {
		return t.keys[i] >= p.Key
	})
	if idx >= len(t.keys) || t.keys[idx] != p.Key {
		return false
	}
	pos := idx
	for i := 0; i < len(p.Path); i++ {
		if pos%2 == 0 {
			if pos+1 < len(t.leaves) {
				cur = nodeHash(cur, mustDecode(p.Path[i]))
			} else {
				cur = nodeHash(cur, cur)
			}
		} else {
			cur = nodeHash(mustDecode(p.Path[i]), cur)
		}
		pos /= 2
	}
	return hex.EncodeToString(cur) == t.Root()
}

func mustDecode(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}
