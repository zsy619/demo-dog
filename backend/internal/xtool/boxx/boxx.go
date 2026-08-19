// Package boxx 提供类型安全容器 / Box 包装。
package boxx

// Box 是一个泛型容器。
type Box[T any] struct {
	v T
}

// New 用给定值构造 Box。
func New[T any](v T) Box[T] { return Box[T]{v: v} }

// Value 返回 Box 中的值。
func (b Box[T]) Value() T { return b.v }

// Set 设置 Box 中的值。
func (b *Box[T]) Set(v T) { b.v = v }

// Map 应用 fn 转换 Box 中的值，返回新 Box。
func Map[T, U any](b Box[T], fn func(T) U) Box[U] {
	return Box[U]{v: fn(b.v)}
}

// OrElse 如果 Box 中的值不满足 ok，返回 default。
func OrElse[T any](b Box[T], ok bool, def T) T {
	if !ok {
		return def
	}
	return b.v
}
