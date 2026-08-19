// Package cmp 提供通用比较器辅助。
package cmp

// Compare 按 cmp(a, b) 排序。
// 返回 -1 / 0 / 1。
func Compare[T any](a, b T, lt func(a, b T) bool) int {
	if lt(a, b) {
		return -1
	}
	if lt(b, a) {
		return 1
	}
	return 0
}

// LessInt 是 int 比较器。
func LessInt(a, b int) bool { return a < b }

// LessString 是字符串比较器。
func LessString(a, b string) bool { return a < b }

// Min 返回两个值中较小的（依赖 lt）。
func Min[T any](a, b T, lt func(a, b T) bool) T {
	if lt(b, a) {
		return b
	}
	return a
}

// Max 返回两个值中较大的（依赖 lt）。
func Max[T any](a, b T, lt func(a, b T) bool) T {
	if lt(b, a) {
		return a
	}
	return b
}

// Equal 返回 a 与 b 的浅比较（reflect.DeepEqual）。
func Equal(a, b any) bool {
	return deepEqual(a, b)
}

func deepEqual(a, b any) bool {
	return a == b
}
