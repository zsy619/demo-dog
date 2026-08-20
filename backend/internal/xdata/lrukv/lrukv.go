// Package lrukv 提供一个 string -> []byte 的并发 LRU 缓存（container/list 实现）。
package lrukv

import (
	"container/list"
	"sync"
)

// KV 是 string->[]byte 的并发 LRU。
type KV struct {
	mu    sync.Mutex
	cap   int
	ll    *list.List
	index map[string]*list.Element
}

type kvEntry struct {
	key string
	val []byte
}

// New 创建容量 cap 的 LRU。
func New(cap int) *KV {
	if cap < 1 {
		cap = 64
	}
	return &KV{
		cap:   cap,
		ll:    list.New(),
		index: make(map[string]*list.Element),
	}
}

// Put 放入键值。
func (k *KV) Put(key string, val []byte) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if el, ok := k.index[key]; ok {
		el.Value.(*kvEntry).val = val
		k.ll.MoveToFront(el)
		return
	}
	if k.ll.Len() >= k.cap {
		back := k.ll.Back()
		if back != nil {
			k.ll.Remove(back)
			delete(k.index, back.Value.(*kvEntry).key)
		}
	}
	el := k.ll.PushFront(&kvEntry{key: key, val: val})
	k.index[key] = el
}

// Get 读取键值。
func (k *KV) Get(key string) ([]byte, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	el, ok := k.index[key]
	if !ok {
		return nil, false
	}
	k.ll.MoveToFront(el)
	return el.Value.(*kvEntry).val, true
}

// Delete 删除键。
func (k *KV) Delete(key string) {
	k.mu.Lock()
	el, ok := k.index[key]
	if ok {
		k.ll.Remove(el)
		delete(k.index, key)
	}
	k.mu.Unlock()
}

// Len 返回元素数。
func (k *KV) Len() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.ll.Len()
}

// Cap 返回容量。
func (k *KV) Cap() int { return k.cap }

// Keys 按访问顺序返回所有 key。
func (k *KV) Keys() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := make([]string, 0, k.ll.Len())
	for e := k.ll.Front(); e != nil; e = e.Next() {
		out = append(out, e.Value.(*kvEntry).key)
	}
	return out
}
