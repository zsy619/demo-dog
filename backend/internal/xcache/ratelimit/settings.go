// Package ratelimit 速率限制：令牌桶与漏桶，支持按 key 隔离。
//
// 两种算法：
//
//   - TokenBucket（令牌桶）：capacity 为突发上限，refill 为稳态速率。
//     典型用途：API 网关限流。
//   - LeakyBucket（漏桶）：积压请求队列，按固定速率排出。
//     典型用途：平滑突发流量。
//
// 两种算法都按 key（租户 ID、IP、API Key ID）分片，最大分片数 MaxShards 限制内存。
//
// 所有公共方法都是 goroutine 安全的。
//
// 本包按类型拆分到多个文件：
//   - settings.go  Settings 配置与默认值解析
//   - bucket.go    Limiter 主体与令牌桶 / 漏桶算法实现
//   - snapshot.go  监控快照类型与快照 API
package ratelimit

// settings.go：Settings 配置与默认值解析。
//
// 所有零值都会自动回退到合理默认值；Now 用于测试注入时间源。

import (
	"errors"
	"time"
)

// ErrLimited 在请求被限流时返回。
//
// 调用方应使用 errors.Is(err, ratelimit.ErrLimited) 判断。
var ErrLimited = errors.New("rate limit exceeded")

// Settings 配置单个 Limiter。
//
// 所有字段都允许零值，会自动回退到默认值；
// Now 主要用于测试时注入虚拟时钟。
type Settings struct {
	Capacity     int             // 令牌桶容量（突发上限）
	RefillPerSec float64         // 令牌桶每秒补充速率
	LeakPerSec   float64         // 漏桶每秒漏出速率
	MaxShards    int             // 最大分片数（超过则新 key 被拒绝）
	Now          func() time.Time // 时间源（注入用于测试）
}

// now 返回 Settings.Now 或默认 time.Now。
func (s *Settings) now() func() time.Time {
	if s.Now == nil {
		return time.Now
	}
	return s.Now
}

// capacity 返回有效容量（默认 100）。
func (s *Settings) capacity() int {
	if s.Capacity <= 0 {
		return 100
	}
	return s.Capacity
}

// refill 返回有效令牌补充速率（默认 10 tokens/s）。
func (s *Settings) refill() float64 {
	if s.RefillPerSec <= 0 {
		return 10
	}
	return s.RefillPerSec
}

// leak 返回有效漏桶漏出速率（默认 10 req/s）。
func (s *Settings) leak() float64 {
	if s.LeakPerSec <= 0 {
		return 10
	}
	return s.LeakPerSec
}

// maxShards 返回有效最大分片数（默认 10_000）。
func (s *Settings) maxShards() int {
	if s.MaxShards <= 0 {
		return 10_000
	}
	return s.MaxShards
}
