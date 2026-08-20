package breaker

import "time"

// outcome 记录单次调用的结果：是否成功与发生时间。
//
// 用于内部滑动窗口评估；不是公共 API。
type outcome struct {
	success bool      // 调用是否成功
	at      time.Time // 调用完成时间
}
