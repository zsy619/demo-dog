package retry

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      float64
	IsRetryable func(error) bool
	OnRetry     func(attempt int, err error, next time.Duration)
}

func Default() Config {
	return Config{
		MaxAttempts: 5,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Jitter:      0.2,
		IsRetryable: func(error) bool { return true },
	}
}

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
	if cfg.IsRetryable == nil {
		cfg.IsRetryable = func(error) bool { return true }
	}
	var lastErr error
	delay := cfg.BaseDelay
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		err := op(ctx)
		if err == nil {
			return nil
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
		delay = nextDelay(delay, cfg.MaxDelay)
	}
	return &RetryError{Attempts: cfg.MaxAttempts, Err: lastErr}
}

type RetryError struct {
	Attempts int
	Err      error
}

func (e *RetryError) Error() string {
	return "retry: " + itoa(e.Attempts) + " attempts failed: " + e.Err.Error()
}

func (e *RetryError) Unwrap() error { return e.Err }

func IsRetryableError(err error) bool {
	var r *RetryError
	return errors.As(err, &r)
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

func nextDelay(d, max time.Duration) time.Duration {
	n := d * 2
	if n > max {
		return max
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
