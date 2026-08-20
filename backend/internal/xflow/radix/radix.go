// Package radix Radix 树：最长前缀匹配，支持压缩节点。
package radix

import (
	"errors"
	"strings"
	"sync"
)

// Tree is a radix tree for string keys.
type Tree struct {
	mu   sync.RWMutex
	root *node
}

type node struct {
	prefix   string
	children map[string]*node
	value    any
	leaf     bool
}

func newNode(prefix string) *node {
	return &node{prefix: prefix, children: make(map[string]*node)}
}

// New 创建一个空 Tree。
func New() *Tree {
	return &Tree{root: newNode("")}
}

// Insert 插入 key 与 value。若 key 已存在则覆盖。
func (t *Tree) Insert(key string, value any) {
	if key == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	insert(t.root, key, value)
}

func insert(n *node, key string, value any) {
	for {
		// 查找 n.prefix 与 key 之间的最长前缀匹配。
		common := commonPrefix(n.prefix, key)
		switch {
		case common == n.prefix && len(key) == len(common):
			// 对 n 的精确匹配。设置 leaf + value。
			n.leaf = true
			n.value = value
			return
		case common == n.prefix:
			// n.prefix fully matched, descend.
			key = key[len(common):]
			first := string(key[0])
			child, ok := n.children[first]
			if !ok {
				child = newNode(key)
				n.children[first] = child
			}
			n = child
		default:
			// 分裂 n。n.prefix = 公共前缀；新子节点的 n.prefix[len(common):]。
			oldPrefix := n.prefix
			oldChildren := n.children
			oldLeaf := n.leaf
			oldValue := n.value
			// 就地变更 n 成为分裂节点。
			n.prefix = common
			// 在新的兄弟节点中保留旧的子节点 + 状态。
			sibling := newNode(oldPrefix[len(common):])
			sibling.children = oldChildren
			sibling.leaf = oldLeaf
			sibling.value = oldValue
			n.children = make(map[string]*node)
			n.children[string(sibling.prefix[0])] = sibling
			key = key[len(common):]
			if key == "" {
				n.leaf = true
				n.value = value
				return
			}
			first := string(key[0])
			child := newNode(key)
			n.children[first] = child
			n = child
		}
	}
}

// Lookup returns the value for key, or nil if not present.
func (t *Tree) Lookup(key string) any {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := t.root
	for {
		if !strings.HasPrefix(key, n.prefix) {
			return nil
		}
		key = key[len(n.prefix):]
		if key == "" {
			if n.leaf {
				return n.value
			}
			return nil
		}
		child, ok := n.children[string(key[0])]
		if !ok {
			return nil
		}
		n = child
	}
}

// ErrBadPattern 在通配符模式非法时返回。
var ErrBadPattern = errors.New("bad pattern")

// MatchPattern looks up a pattern that may end with a "*"
// wildcard. Returns the value and whether the prefix matched.
func (t *Tree) MatchPattern(pattern string) (any, bool) {
	star := strings.Index(pattern, "*")
	if star < 0 {
		v := t.Lookup(pattern)
		return v, v != nil
	}
	prefix := pattern[:star]
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := lookupNode(t.root, prefix)
	if n == nil || !n.leaf {
		return nil, false
	}
	return n.value, true
}

func lookupNode(n *node, key string) *node {
	for {
		if !strings.HasPrefix(key, n.prefix) {
			return nil
		}
		key = key[len(n.prefix):]
		if key == "" {
			return n
		}
		child, ok := n.children[string(key[0])]
		if !ok {
			return nil
		}
		n = child
	}
}

// Len 返回叶子条目数。
func (t *Tree) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return countLeaves(t.root)
}

func countLeaves(n *node) int {
	if n == nil {
		return 0
	}
	nl := 0
	if n.leaf {
		nl = 1
	}
	for _, c := range n.children {
		nl += countLeaves(c)
	}
	return nl
}

func commonPrefix(a, b string) string {
	max := len(a)
	if len(b) < max {
		max = len(b)
	}
	for i := 0; i < max; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:max]
}
