package breaker

import (
	"errors"
	"time"
)

// Config 描述断路器的所有阈值参数。
//
// 字段语义：
//   - Window：滑动窗口大小，超出此时间的结果将被丢弃；
//   - MinSamples：触发评估所需的最小调用次数；
//   - FailureRatio：失败比例阈值，达到则熔断；
//   - OpenTimeout：熔断后等待时长，到期后转入半开；
//   - HalfOpenCalls：半开状态下允许通过的探测调用数。
type Config struct {
	Window        time.Duration // 滑动窗口大小
	MinSamples    int           // 触发评估的最小样本数
	FailureRatio  float64       // 失败率阈值（0-1）
	OpenTimeout   time.Duration // 熔断持续时间
	HalfOpenCalls int           // 半开状态允许的探测调用数
}

// ErrOpen 在 Allow() 返回 false 时作为 ShortCircuit() 的返回值。
var ErrOpen = errors.New("breaker open")
