package retry

import (
	"strconv"
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_FirstSuccess(t *testing.T) {
	err := Do(context.Background(), Default(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDo_Exhausted(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    20 * time.Millisecond,
		Jitter:      0,
		IsRetryable: func(error) bool { return true },
	}, func(ctx context.Context) error {
		attempts++
		return errors.New("nope")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 3 {
		t.Fatalf("attempts: %d", attempts)
	}
	if !IsRetryableError(err) {
		t.Fatal("should be RetryError")
	}
}

func TestDo_SucceedsAfterRetries(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    20 * time.Millisecond,
		Jitter:      0,
		IsRetryable: func(error) bool { return true },
	}, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts: %d", attempts)
	}
}

func TestDo_NonRetryable(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    20 * time.Millisecond,
		Jitter:      0,
		IsRetryable: func(e error) bool { return e.Error() != "fatal" },
	}, func(ctx context.Context) error {
		attempts++
		return errors.New("fatal")
	})
	if err == nil || err.Error() != "fatal" {
		t.Fatal("should return fatal")
	}
	if attempts != 1 {
		t.Fatalf("attempts: %d", attempts)
	}
}

func TestDo_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Do(ctx, Default(), func(ctx context.Context) error {
		return errors.New("x")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("expected canceled")
	}
}

func TestDo_OnRetry(t *testing.T) {
	calls := 0
	Do(context.Background(), Config{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    20 * time.Millisecond,
		Jitter:      0,
		IsRetryable: func(error) bool { return true },
		OnRetry: func(attempt int, err error, next time.Duration) {
			calls++
		},
	}, func(ctx context.Context) error {
		return errors.New("x")
	})
	if calls != 2 {
		t.Fatalf("onretry: %d", calls)
	}
}

func TestNextDelay(t *testing.T) {
	d := nextDelay(100*time.Millisecond, time.Second, 2.0)
	if d != 200*time.Millisecond {
		t.Fatal(d)
	}
	d = nextDelay(800*time.Millisecond, time.Second, 2.0)
	if d != time.Second {
		t.Fatal(d)
	}
}

func TestAddJitter_NoJitter(t *testing.T) {
	if d := addJitter(time.Second, 0); d != time.Second {
		t.Fatal(d)
	}
}

func TestAddJitter_Bounded(t *testing.T) {
	d := addJitter(time.Second, 0.5)
	if d < 500*time.Millisecond || d > 1500*time.Millisecond {
		t.Fatalf("out of range: %v", d)
	}
}

func TestItoa(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{123, "123"},
		{-7, "-7"},
	}
	for _, c := range cases {
		if got := strconv.Itoa(c.in); got != c.want {
			t.Fatalf("itoa(%d): %s want %s", c.in, got, c.want)
		}
	}
}

func TestRetryError_Unwrap(t *testing.T) {
	e := &RetryError{Attempts: 3, Err: errors.New("inner")}
	var inner *RetryError
	if !errors.As(e, &inner) {
		t.Fatal("unwrap")
	}
	if inner.Err.Error() != "inner" {
		t.Fatal("inner msg")
	}
	if !errors.Is(e, e.Err) {
		t.Fatal("Is")
	}
}

func TestDefault_Values(t *testing.T) {
	d := Default()
	if d.MaxAttempts <= 0 || d.BaseDelay <= 0 || d.MaxDelay <= 0 {
		t.Fatal("defaults should be set")
	}
}
