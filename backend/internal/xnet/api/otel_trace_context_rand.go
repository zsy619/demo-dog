package api

import "crypto/rand"

// cryptoRandBytes 对 crypto/rand.Read 做了包装，让 trace context 相关代码
// 保持精简的 import 依赖面。
func cryptoRandBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// 根据文档，crypto/rand 在 stdlib 中永远不会失败，除非
		// 遭遇灾难性的熵源枯竭；在那种情况下
		// 返回零字节是合理的降级行为：trace
		// context 在语法上仍然合法，下游
		// 工具会把它当作 "unknown" 处理。
		return b
	}
	return b
}
