// Package backoff 提供指数退避与抖动重试策略：
// 适合网络/IO 操作的有限重试。
package backoff

import (
	"context"
	"math/rand"
	"time"
)

// Strategy 是退避参数。
type Strategy struct {
	Base     time.Duration // 初始退避
	Max      time.Duration // 单次退避上限
	Jitter   float64       // 抖动比例 [0, 1]
	Attempts int           // 最大尝试次数（不含第一次）
}

// Default 返回默认退避配置。
func Default() Strategy {
	return Strategy{Base: 50 * time.Millisecond, Max: 5 * time.Second, Jitter: 0.2, Attempts: 5}
}

// Next 返回第 n 次（从 0 开始）退避时间。
func (s Strategy) Next(n int) time.Duration {
	d := s.Base << n
	if d <= 0 || d > s.Max {
		d = s.Max
	}
	if s.Jitter > 0 {
		d += time.Duration(float64(d) * s.Jitter * (2*rand.Float64() - 1))
		if d < 0 {
			d = 0
		}
	}
	return d
}

// Do 在 ctx 下以退避策略重试 fn，直到 fn 返回 nil 或 ctx 取消。
// fn 返回 error 时，fn 返回 nil 时停止重试。
func (s Strategy) Do(ctx context.Context, fn func() error) error {
	var err error
	for i := 0; i <= s.Attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if i == s.Attempts {
			break
		}
		wait := s.Next(i)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return err
}
