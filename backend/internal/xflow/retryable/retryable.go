// Package retryable 提供带指数退避的可重试任务执行。
// 它支持最大尝试次数、自定义退避、错误分类（是否可重试）。
package retryable

import (
	"errors"
	"math"
	"math/rand"
	"time"
)

// Policy 描述重试策略。
type Policy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      bool
}

// Default 返回默认策略。
func Default() Policy {
	return Policy{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
		Jitter:      true,
	}
}

// ErrMaxAttempts 在超过最大重试次数后返回。
var ErrMaxAttempts = errors.New("retryable: 已达最大重试次数")

// Result 描述单次调用的结果。
type Result struct {
	Attempts int
	Err      error
	Elapsed  time.Duration
}

// Do 执行 fn，按 policy 重试。fn 返回 nil 表示成功；
// 返回 PermanentError 则立即终止；其他错误尝试重试。
func Do(p Policy, fn func() error) Result {
	start := time.Now()
	if p.MaxAttempts <= 0 {
		p = Default()
	}
	var last error
	for i := 1; i <= p.MaxAttempts; i++ {
		err := fn()
		if err == nil {
			return Result{Attempts: i, Err: nil, Elapsed: time.Since(start)}
		}
		var perm *PermanentError
		if errors.As(err, &perm) {
			return Result{Attempts: i, Err: err, Elapsed: time.Since(start)}
		}
		last = err
		if i == p.MaxAttempts {
			break
		}
		time.Sleep(delayFor(p, i))
	}
	return Result{Attempts: p.MaxAttempts, Err: errors.Join(last, ErrMaxAttempts), Elapsed: time.Since(start)}
}

// PermanentError 标记不可重试错误。
type PermanentError struct {
	Err error
}

func (p *PermanentError) Error() string { return p.Err.Error() }
func (p *PermanentError) Unwrap() error { return p.Err }

// Permanent 包装一个永久错误。
func Permanent(err error) error { return &PermanentError{Err: err} }

func delayFor(p Policy, attempt int) time.Duration {
	d := float64(p.BaseDelay) * math.Pow(p.Multiplier, float64(attempt-1))
	if d > float64(p.MaxDelay) {
		d = float64(p.MaxDelay)
	}
	if p.Jitter {
		d = d * (0.5 + rand.Float64())
	}
	return time.Duration(d)
}
