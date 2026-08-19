// Package optional 提供泛型 Option[T] 类型，区分"未设置"与"零值"。
package optional

// Optional 表示一个可有可无的值。
type Optional[T any] struct {
	v     T
	set   bool
}

// Some 创建一个带值的 Optional。
func Some[T any](v T) Optional[T] {
	return Optional[T]{v: v, set: true}
}

// None 创建一个空 Optional。
func None[T any]() Optional[T] {
	return Optional[T]{}
}

// FromPtr 从指针创建 Optional（nil -> None）。
func FromPtr[T any](p *T) Optional[T] {
	if p == nil {
		return None[T]()
	}
	return Some(*p)
}

// IsPresent 返回是否设值。
func (o Optional[T]) IsPresent() bool { return o.set }

// Value 返回值（未设值时返回零值）。
func (o Optional[T]) Value() T { return o.v }

// OrElse 未设值时返回 def。
func (o Optional[T]) OrElse(def T) T {
	if o.set {
		return o.v
	}
	return def
}

// Map 应用 fn 到当前值。
func Map[T, U any](o Optional[T], fn func(T) U) Optional[U] {
	if !o.set {
		return None[U]()
	}
	return Some(fn(o.v))
}

// Ptr 返回指针（未设值返回 nil）。
func (o Optional[T]) Ptr() *T {
	if !o.set {
		return nil
	}
	return &o.v
}
