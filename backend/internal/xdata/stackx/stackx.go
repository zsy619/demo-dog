// Package stackx 提供线程安全的泛型栈。
package stackx

import "sync"

// Stack 是 LIFO 栈。
type Stack[T any] struct {
	mu sync.Mutex
	d  []T
}

// New 创建一个空栈。
func New[T any]() *Stack[T] {
	return &Stack[T]{}
}

// Push 把元素压入栈。
func (s *Stack[T]) Push(v T) {
	s.mu.Lock()
	s.d = append(s.d, v)
	s.mu.Unlock()
}

// Pop 弹出栈顶元素。
func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.d) == 0 {
		return zero, false
	}
	v := s.d[len(s.d)-1]
	s.d = s.d[:len(s.d)-1]
	return v, true
}

// Peek 查看栈顶元素。
func (s *Stack[T]) Peek() (T, bool) {
	var zero T
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.d) == 0 {
		return zero, false
	}
	return s.d[len(s.d)-1], true
}

// Len 返回栈大小。
func (s *Stack[T]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.d)
}

// Clear 清空。
func (s *Stack[T]) Clear() {
	s.mu.Lock()
	s.d = nil
	s.mu.Unlock()
}
