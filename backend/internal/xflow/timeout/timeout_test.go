package timeout

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_OK(t *testing.T) {
	err := Do(time.Second, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
}

func TestDo_Timeout(t *testing.T) {
	start := time.Now()
	err := Do(20*time.Millisecond, func() error {
		time.Sleep(time.Second)
		return nil
	})
	if !errors.Is(err, ErrTimeout) {
		t.Fatal("应 ErrTimeout")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("未及时返回")
	}
}

func TestDo_FnError(t *testing.T) {
	myErr := errors.New("x")
	err := Do(time.Second, func() error { return myErr })
	if !errors.Is(err, myErr) {
		t.Fatal("应 myErr")
	}
}

func TestDoValue(t *testing.T) {
	v, err := DoValue(100*time.Millisecond, func() (int, error) { return 42, nil })
	if err != nil || v != 42 {
		t.Fatal("val")
	}
}

func TestDoValueCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := DoValueCtx(ctx, time.Second, func() (int, error) { return 1, nil })
	if !errors.Is(err, ErrTimeout) {
		t.Fatal("应 ctx cancel")
	}
}
