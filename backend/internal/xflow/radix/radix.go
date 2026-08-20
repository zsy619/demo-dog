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

// New creates an empty Tree.
func New() *Tree {
	return &Tree{root: newNode("")}
}

// Insert inserts key with value. Existing key is overwritten.
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
		// Find longest prefix match between n.prefix and key.
		common := commonPrefix(n.prefix, key)
		switch {
		case common == n.prefix && len(key) == len(common):
			// Exact match on n. Set leaf + value.
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
			// Split n. n.prefix = common; new child has n.prefix[len(common):].
			oldPrefix := n.prefix
			oldChildren := n.children
			oldLeaf := n.leaf
			oldValue := n.value
			// Mutate n in place to become the split node.
			n.prefix = common
			// Preserve old children + state in a new sibling node.
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

// ErrBadPattern is returned when a wildcard pattern is invalid.
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

// Len returns the number of leaf entries.
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
