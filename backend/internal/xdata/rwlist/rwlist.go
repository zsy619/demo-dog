// Package rwlist 提供读写锁保护的泛型双向链表（基于 container/list）。
package rwlist

import (
	"container/list"
	"sync"
)

// List 是一个线程安全的双向链表。
type List[T any] struct {
	mu sync.RWMutex
	l  *list.List
}

// New 创建一个空 List。
func New[T any]() *List[T] { return &List[T]{l: list.New()} }

// PushFront 头插。
func (l *List[T]) PushFront(v T) {
	l.mu.Lock()
	l.l.PushFront(v)
	l.mu.Unlock()
}

// PushBack 尾插。
func (l *List[T]) PushBack(v T) {
	l.mu.Lock()
	l.l.PushBack(v)
	l.mu.Unlock()
}

// PopFront 头出。
func (l *List[T]) PopFront() (T, bool) {
	var zero T
	l.mu.Lock()
	defer l.mu.Unlock()
	el := l.l.Front()
	if el == nil {
		return zero, false
	}
	l.l.Remove(el)
	return el.Value.(T), true
}

// PopBack 尾出。
func (l *List[T]) PopBack() (T, bool) {
	var zero T
	l.mu.Lock()
	defer l.mu.Unlock()
	el := l.l.Back()
	if el == nil {
		return zero, false
	}
	l.l.Remove(el)
	return el.Value.(T), true
}

// Len 返回长度。
func (l *List[T]) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.l.Len()
}

// Range 按顺序遍历。
func (l *List[T]) Range(fn func(v T) bool) {
	l.mu.RLock()
	snapshot := make([]T, 0, l.l.Len())
	for e := l.l.Front(); e != nil; e = e.Next() {
		snapshot = append(snapshot, e.Value.(T))
	}
	l.mu.RUnlock()
	for _, v := range snapshot {
		if !fn(v) {
			return
		}
	}
}

// Clear 清空。
func (l *List[T]) Clear() {
	l.mu.Lock()
	l.l.Init()
	l.mu.Unlock()
}
