// Package sets 提供一组集合操作辅助（基于 map）。
package sets

// Set 是一个 string 集合。
type Set map[string]struct{}

// New 创建一个包含 initial 的集合。
func New(initial ...string) Set {
	s := make(Set, len(initial))
	for _, v := range initial {
		s[v] = struct{}{}
	}
	return s
}

// Add 添加元素。
func (s Set) Add(v ...string) {
	for _, x := range v {
		s[x] = struct{}{}
	}
}

// Remove 删除元素。
func (s Set) Remove(v ...string) {
	for _, x := range v {
		delete(s, x)
	}
}

// Has 判断是否包含。
func (s Set) Has(v string) bool {
	_, ok := s[v]
	return ok
}

// Len 返回元素数。
func (s Set) Len() int { return len(s) }

// Slice 返回元素切片（顺序随机）。
func (s Set) Slice() []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return out
}

// Union 返回并集。
func Union(a, b Set) Set {
	out := make(Set, len(a)+len(b))
	for k := range a {
		out[k] = struct{}{}
	}
	for k := range b {
		out[k] = struct{}{}
	}
	return out
}

// Intersect 返回交集。
func Intersect(a, b Set) Set {
	out := make(Set)
	for k := range a {
		if b.Has(k) {
			out[k] = struct{}{}
		}
	}
	return out
}

// Diff 返回 a-b 差集。
func Diff(a, b Set) Set {
	out := make(Set)
	for k := range a {
		if !b.Has(k) {
			out[k] = struct{}{}
		}
	}
	return out
}
