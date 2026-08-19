// Package timeout 提供一个超时装饰器：包装一个阻塞函数，
// 在指定时长后放弃等待并返回错误。
package timeout

import (
	"context"
	"errors"
	"time"
)

// ErrTimeout 在操作超过指定时长时返回。
var ErrTimeout = errors.New("timeout: 超时")

// Do 在 d 时间后放弃执行 fn（实际 fn 仍可能继续运行）。
func Do(d time.Duration, fn func() error) error {
	return DoCtx(context.Background(), d, fn)
}

// DoCtx 在 ctx 或 d 触发后放弃等待。
func DoCtx(ctx context.Context, d time.Duration, fn func() error) error {
	c, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- ErrTimeout
			}
		}()
		done <- fn()
	}()
	select {
	case <-c.Done():
		return ErrTimeout
	case err := <-done:
		return err
	}
}

// DoValue 是 Do 的泛型版本。
func DoValue[T any](d time.Duration, fn func() (T, error)) (T, error) {
	return DoValueCtx(context.Background(), d, fn)
}

// DoValueCtx 是带 ctx 的泛型版本。
func DoValueCtx[T any](ctx context.Context, d time.Duration, fn func() (T, error)) (T, error) {
	var zero T
	c, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := fn()
		done <- result{v: v, err: err}
	}()
	select {
	case <-c.Done():
		return zero, ErrTimeout
	case r := <-done:
		return r.v, r.err
	}
}
