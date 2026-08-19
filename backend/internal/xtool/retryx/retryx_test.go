package retryx

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	p := Default()
	calls := 0
	err := p.Do(context.Background(), func(_ context.Context) error {
		calls++
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatal("succ")
	}
}

func TestDo_Retry(t *testing.T) {
	p := Policy{Attempts: 3, Delay: 1 * time.Millisecond, RetryIf: func(err error) bool { return err != nil }}
	calls := 0
	p.Do(context.Background(), func(_ context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("x")
		}
		return nil
	})
	if calls != 3 {
		t.Fatal("retry")
	}
}

func TestDo_NoRetry(t *testing.T) {
	errFatal := errors.New("fatal")
	p := Policy{Attempts: 5, Delay: 1 * time.Millisecond, RetryIf: func(err error) bool { return errors.Is(err, errFatal) }}
	err := p.Do(context.Background(), func(_ context.Context) error { return errFatal })
	if !errors.Is(err, errFatal) {
		t.Fatal("fatal")
	}
}

func TestDo_Exceeds(t *testing.T) {
	p := Policy{Attempts: 2, Delay: 1 * time.Millisecond}
	err := p.Do(context.Background(), func(_ context.Context) error { return errors.New("x") })
	if err == nil {
		t.Fatal("应失败")
	}
}

func TestDoValue(t *testing.T) {
	p := Default()
	v, err := DoValue(context.Background(), p, func(_ context.Context) (int, error) {
		return 42, nil
	})
	if err != nil || v != 42 {
		t.Fatal("val")
	}
}

func TestWrap(t *testing.T) {
	p := Default()
	f := Wrap(p, func(_ context.Context) error { return nil })
	if err := f(context.Background()); err != nil {
		t.Fatal("wrap")
	}
}
