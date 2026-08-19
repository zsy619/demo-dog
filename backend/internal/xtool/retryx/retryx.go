// Package retryx 提供函数包装式的重试装饰器。
package retryx

import (
	"context"
	"errors"
	"time"
)

// Policy 重试策略。
type Policy struct {
	Attempts int
	Delay    time.Duration
	MaxDelay time.Duration
	RetryIf  func(error) bool
}

// Default 返回默认策略。
func Default() Policy {
	return Policy{Attempts: 3, Delay: 100 * time.Millisecond, MaxDelay: time.Second, RetryIf: func(err error) bool { return err != nil }}
}

// ErrExceededAttempts 在达到最大尝试次数后返回。
var ErrExceededAttempts = errors.New("retryx: 已达最大尝试次数")

// Do 按策略重试 fn；最后一次的错误会包装为 ErrExceededAttempts。
func (p Policy) Do(ctx context.Context, fn func(context.Context) error) error {
	if p.Attempts <= 0 {
		p.Attempts = 1
	}
	delay := p.Delay
	var lastErr error
	for i := 0; i < p.Attempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := fn(ctx)
		if err == nil {
			return nil
		}
		if p.RetryIf != nil && !p.RetryIf(err) {
			return err
		}
		lastErr = err
		if i == p.Attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
		if p.MaxDelay > 0 && delay > p.MaxDelay {
			delay = p.MaxDelay
		}
	}
	if lastErr == nil {
		return ErrExceededAttempts
	}
	return lastErr
}

// Wrap 把 fn 包成带重试策略的版本。
func Wrap(p Policy, fn func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		return p.Do(ctx, fn)
	}
}

// DoValue 是 Do 的泛型返回值版本。
func DoValue[T any](ctx context.Context, p Policy, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if p.Attempts <= 0 {
		p.Attempts = 1
	}
	delay := p.Delay
	for i := 0; i < p.Attempts; i++ {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		v, err := fn(ctx)
		if err == nil {
			return v, nil
		}
		if p.RetryIf != nil && !p.RetryIf(err) {
			return zero, err
		}
		if i == p.Attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
		if p.MaxDelay > 0 && delay > p.MaxDelay {
			delay = p.MaxDelay
		}
	}
	return zero, ErrExceededAttempts
}
