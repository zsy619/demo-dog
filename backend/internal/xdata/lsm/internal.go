package lsm

// internal.go:私有辅助。

// copyBytes 深拷贝一个字节切片(nil 安全)。
func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
