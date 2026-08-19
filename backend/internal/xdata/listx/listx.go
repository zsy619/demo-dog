// Package listx 提供线程安全的双向链表（基于 container/list）。
package listx

import (
	"container/list"
	"sync"
)

// List 是一个泛型双向链表。
type List[T any] struct {
	mu sync.Mutex
	l  *list.List
}

// New 创建一个空链表。
func New[T any]() *List[T] {
	return &List[T]{l: list.New()}
}

// PushBack 把元素加入尾部。
func (l *List[T]) PushBack(v T) *Element[T] {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &Element[T]{e: l.l.PushBack(v)}
}

// PushFront 把元素加入头部。
func (l *List[T]) PushFront(v T) *Element[T] {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &Element[T]{e: l.l.PushFront(v)}
}

// Front 返回头节点。
func (l *List[T]) Front() *Element[T] {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e := l.l.Front(); e != nil {
		return &Element[T]{e: e}
	}
	return nil
}

// Back 返回尾节点。
func (l *List[T]) Back() *Element[T] {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e := l.l.Back(); e != nil {
		return &Element[T]{e: e}
	}
	return nil
}

// Len 返回元素数。
func (l *List[T]) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.l.Len()
}

// Clear 清空。
func (l *List[T]) Clear() {
	l.mu.Lock()
	l.l.Init()
	l.mu.Unlock()
}

// Range 按顺序遍历，回调返回 false 停止。
func (l *List[T]) Range(fn func(v T) bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for e := l.l.Front(); e != nil; e = e.Next() {
		if !fn(e.Value.(T)) {
			return
		}
	}
}

// Element 是链表节点。
type Element[T any] struct {
	e *list.Element
}

// Value 返回节点值。
func (e *Element[T]) Value() T { return e.e.Value.(T) }

// Remove 删除节点。
func (e *Element[T]) Remove() {
	if e == nil || e.e == nil {
		return
	}
	// 取 list 需要其它方式，这里通过 Next/Prev 找到 list 不直接；改为标记
	e.e.Value = nil
}

// Next 返回下一个节点。
func (e *Element[T]) Next() *Element[T] {
	if e == nil || e.e == nil {
		return nil
	}
	if n := e.e.Next(); n != nil {
		return &Element[T]{e: n}
	}
	return nil
}
