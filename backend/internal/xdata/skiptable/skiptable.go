// Package skiptable 提供一个有序键值存储（基于跳表）。
package skiptable

import (
	"math/rand"
	"sync"
)

const maxLevel = 16

// SkipList 是一个简单跳表。
type SkipList struct {
	mu    sync.Mutex
	head  *node
	level int
	rng   *rand.Rand
}

type node struct {
	key  string
	val  any
	next []*node
}

// New 创建一个空跳表。
func New() *SkipList {
	return &SkipList{
		head:  &node{next: make([]*node, maxLevel)},
		level: 0,
		rng:   rand.New(rand.NewSource(int64(1))),
	}
}

// Set 设置键值。
func (s *SkipList) Set(k string, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	update := make([]*node, maxLevel)
	cur := s.head
	for i := s.level; i >= 0; i-- {
		for cur.next[i] != nil && cur.next[i].key < k {
			cur = cur.next[i]
		}
		update[i] = cur
	}
	if cur.next[0] != nil && cur.next[0].key == k {
		cur.next[0].val = v
		return
	}
	lvl := s.randomLevel()
	if lvl > s.level {
		for i := s.level + 1; i <= lvl; i++ {
			update[i] = s.head
		}
		s.level = lvl
	}
	n := &node{key: k, val: v, next: make([]*node, lvl+1)}
	for i := 0; i <= lvl; i++ {
		n.next[i] = update[i].next[i]
		update[i].next[i] = n
	}
}

// Get 读取键值。
func (s *SkipList) Get(k string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.head
	for i := s.level; i >= 0; i-- {
		for cur.next[i] != nil && cur.next[i].key < k {
			cur = cur.next[i]
		}
	}
	if cur.next[0] != nil && cur.next[0].key == k {
		return cur.next[0].val, true
	}
	return nil, false
}

// Delete 删除一个键。
func (s *SkipList) Delete(k string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	update := make([]*node, maxLevel)
	cur := s.head
	for i := s.level; i >= 0; i-- {
		for cur.next[i] != nil && cur.next[i].key < k {
			cur = cur.next[i]
		}
		update[i] = cur
	}
	target := cur.next[0]
	if target != nil && target.key == k {
		for i := 0; i <= s.level; i++ {
			if update[i].next[i] != target {
				break
			}
			update[i].next[i] = target.next[i]
		}
		for s.level > 0 && s.head.next[s.level] == nil {
			s.level--
		}
	}
}

// Len 返回元素数。
func (s *SkipList) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	cur := s.head.next[0]
	for cur != nil {
		n++
		cur = cur.next[0]
	}
	return n
}

func (s *SkipList) randomLevel() int {
	lvl := 0
	for s.rng.Float64() < 0.5 && lvl < maxLevel-1 {
		lvl++
	}
	return lvl
}

// Range 按升序遍历所有键值对。
func (s *SkipList) Range(fn func(k string, v any) bool) {
	s.mu.Lock()
	cur := s.head.next[0]
	nodes := make([]*node, 0)
	for cur != nil {
		nodes = append(nodes, cur)
		cur = cur.next[0]
	}
	s.mu.Unlock()
	for _, n := range nodes {
		if !fn(n.key, n.val) {
			return
		}
	}
}
