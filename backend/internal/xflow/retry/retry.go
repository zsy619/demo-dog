// Package retry 提供带指数退避与抖动的可重试执行：
// 支持最大尝试次数、自定义 IsRetryable、PermanentError 立即终止、
// OnRetry 回调、泛型值返回。
package retry

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"time"
)

// Config 重试配置。
type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      float64 // 0..1 抖动比例
	IsRetryable func(error) bool
	OnRetry     func(attempt int, err error, next time.Duration)
}

// Default 返回默认策略。
func Default() Config {
	return Config{
		MaxAttempts: 5,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
		Jitter:      0.2,
		IsRetryable: func(error) bool { return true },
	}
}

// RetryError 描述最终失败的尝试。
type RetryError struct {
	Attempts int
	Err      error
}

func (e *RetryError) Error() string {
	return "retry: " + strconv.Itoa(e.Attempts) + " 次尝试后失败: " + e.Err.Error()
}
func (e *RetryError) Unwrap() error { return e.Err }

// IsRetryableError 判断 err 是否为 RetryError。
func IsRetryableError(err error) bool {
	var r *RetryError
	return errors.As(err, &r)
}

// PermanentError 标记不可重试错误，立即终止重试循环。
type PermanentError struct {
	Err error
}

func (p *PermanentError) Error() string { return p.Err.Error() }
func (p *PermanentError) Unwrap() error { return p.Err }

// Permanent 包装一个永久错误。
func Permanent(err error) error { return &PermanentError{Err: err} }

// Do 按 cfg 重试 op；最后一次的错误会被包装为 RetryError。
// 收到 PermanentError 立即终止（不重试）。
func Do(ctx context.Context, cfg Config, op func(ctx context.Context) error) error {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 50 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = cfg.BaseDelay * 100
	}
	if cfg.Multiplier <= 1 {
		cfg.Multiplier = 2.0
	}
	if cfg.IsRetryable == nil {
		cfg.IsRetryable = func(error) bool { return true }
	}
	var lastErr error
	delay := cfg.BaseDelay
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := op(ctx)
		if err == nil {
			return nil
		}
		// PermanentError 立即终止
		var perm *PermanentError
		if errors.As(err, &perm) {
			return err
		}
		lastErr = err
		if attempt == cfg.MaxAttempts {
			break
		}
		if !cfg.IsRetryable(err) {
			return err
		}
		wait := addJitter(delay, cfg.Jitter)
		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt, err, wait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		delay = nextDelay(delay, cfg.MaxDelay, cfg.Multiplier)
	}
	return &RetryError{Attempts: cfg.MaxAttempts, Err: lastErr}
}

// DoValue 是 Do 的泛型返回值版本。
func DoValue[T any](ctx context.Context, cfg Config, op func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	err := Do(ctx, cfg, func(ctx context.Context) error {
		_, e := op(ctx)
		return e
	})
	if err == nil {
		return zero, nil
	}
	return zero, err
}

// Wrap 把 op 包成带重试策略的版本。
func Wrap(cfg Config, op func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error { return Do(ctx, cfg, op) }
}

func addJitter(d time.Duration, j float64) time.Duration {
	if j <= 0 {
		return d
	}
	delta := time.Duration(float64(d) * j)
	if delta <= 0 {
		return d
	}
	return d + time.Duration(rand.Int63n(int64(delta*2)))-delta
}

func nextDelay(d, max time.Duration, mult float64) time.Duration {
	n := time.Duration(float64(d) * mult)
	if n > max {
		return max
	}
	return n
}
