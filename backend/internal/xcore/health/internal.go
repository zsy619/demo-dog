package health

// internal.go:私有辅助函数。

// removeString 从字符串切片中移除所有等于 t 的元素(原地复用底层数组)。
func removeString(s []string, t string) []string {
	out := s[:0]
	for _, x := range s {
		if x != t {
			out = append(out, x)
		}
	}
	return out
}
