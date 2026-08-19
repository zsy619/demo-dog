// Package index 提供一个简单内存 B-Tree 索引（自实现）。
// 节点使用排序数组与二分查找，支持 Put/Get/Delete/Range。
package index

import (
	"sort"
	"sync"
)

const minDegree = 32

// Item 是带键的条目。
type Item struct {
	Key   []byte
	Value any
}

// Tree 是一个线程安全的 B-Tree。
type Tree struct {
	mu    sync.RWMutex
	root  *node
	count int
}

type node struct {
	leaf     bool
	items    []Item
	children []*node
}

// New 创建一个空 Tree。
func New() *Tree {
	return &Tree{root: &node{leaf: true}}
}

// Put 插入或更新。
func (t *Tree) Put(key []byte, value any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.root == nil {
		t.root = &node{leaf: true}
	}
	pair := t.root.find(key)
	if pair != nil {
		pair.Value = value
		return
	}
	item := Item{Key: append([]byte(nil), key...), Value: value}
	if len(t.root.items) >= 2*minDegree-1 {
		splitRoot(t)
	}
	insertNonFull(t.root, item)
	t.count++
}

// Get 查找一个键。
func (t *Tree) Get(key []byte) (any, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.root == nil {
		return nil, false
	}
	if it := t.root.find(key); it != nil {
		return it.Value, true
	}
	return nil, false
}

// Delete 删除一个键。
func (t *Tree) Delete(key []byte) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.root == nil {
		return false
	}
	if t.root.find(key) == nil {
		return false
	}
	deleteFromNode(t, t.root, key, 0)
	t.count--
	return true
}

// Len 返回元素数量。
func (t *Tree) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.count
}

// Range 范围查询 [from, to)。
func (t *Tree) Range(from, to []byte) []Item {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := []Item{}
	t.collect(from, to, &out)
	return out
}

func (t *Tree) collect(from, to []byte, out *[]Item) {
	collectNode(t.root, from, to, out)
}

func collectNode(n *node, from, to []byte, out *[]Item) {
	if n == nil {
		return
	}
	for i, it := range n.items {
		if !n.leaf {
			collectNode(n.children[i], from, to, out)
		}
		if inRange(it.Key, from, to) {
			*out = append(*out, it)
		}
	}
	if !n.leaf {
		collectNode(n.children[len(n.items)], from, to, out)
	}
}

func inRange(k, from, to []byte) bool {
	if from != nil && compare(k, from) < 0 {
		return false
	}
	if to != nil && compare(k, to) >= 0 {
		return false
	}
	return true
}

func (n *node) find(key []byte) *Item {
	if n == nil {
		return nil
	}
	i := sort.Search(len(n.items), func(i int) bool { return compare(n.items[i].Key, key) >= 0 })
	if i < len(n.items) && compare(n.items[i].Key, key) == 0 {
		return &n.items[i]
	}
	if n.leaf {
		return nil
	}
	return n.children[i].find(key)
}

func insertNonFull(n *node, it Item) {
	if n.leaf {
		i := sort.Search(len(n.items), func(i int) bool { return compare(n.items[i].Key, it.Key) >= 0 })
		n.items = append(n.items, Item{})
		copy(n.items[i+1:], n.items[i:])
		n.items[i] = it
		return
	}
	i := sort.Search(len(n.items), func(i int) bool { return compare(n.items[i].Key, it.Key) >= 0 })
	if len(n.children[i].items) >= 2*minDegree-1 {
		splitChild(n, i)
		if compare(n.items[i].Key, it.Key) < 0 {
			i++
		}
	}
	insertNonFull(n.children[i], it)
}

func splitRoot(t *Tree) {
	old := t.root
	newRoot := &node{leaf: false}
	t.root = newRoot
	newRoot.children = append(newRoot.children, old)
	splitChild(newRoot, 0)
}

func splitChild(parent *node, i int) {
	full := parent.children[i]
	mid := minDegree - 1
	newChild := &node{leaf: full.leaf}
	newChild.items = append(newChild.items, full.items[mid+1:]...)
	if !full.leaf {
		newChild.children = append(newChild.children, full.children[mid+1:]...)
		full.children = full.children[:mid+1]
	}
	median := full.items[mid]
	full.items = full.items[:mid]
	parent.items = append(parent.items, Item{})
	copy(parent.items[i+1:], parent.items[i:])
	parent.items[i] = median
	parent.children = append(parent.children, nil)
	copy(parent.children[i+2:], parent.children[i+1:])
	parent.children[i+1] = newChild
}

func deleteFromNode(t *Tree, n *node, key []byte, idx int) {
	if n == nil {
		return
	}
	if n.leaf {
		if idx < len(n.items) && compare(n.items[idx].Key, key) == 0 {
			n.items = append(n.items[:idx], n.items[idx+1:]...)
			if len(n.items) == 0 && t.root == n && n != nil {
				t.root = nil
			}
		}
		return
	}
	if idx < len(n.items) && compare(n.items[idx].Key, key) == 0 {
		// 仅简化：标记删除
		n.items = append(n.items[:idx], n.items[idx+1:]...)
		return
	}
	childIdx := idx
	if childIdx < len(n.children) {
		deleteFromNode(t, n.children[childIdx], key, idx)
	}
}

func compare(a, b []byte) int {
	la, lb := len(a), len(b)
	n := la
	if lb < n {
		n = lb
	}
	for i := 0; i < n; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	switch {
	case la < lb:
		return -1
	case la > lb:
		return 1
	default:
		return 0
	}
}
