package alerts

// internal.go:私有辅助。

// errString 把 error 转为字符串;nil 返回 ""。
func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}
