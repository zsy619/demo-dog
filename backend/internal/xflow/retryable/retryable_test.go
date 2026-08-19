package retryable

import (
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	p := Policy{MaxAttempts: 3, BaseDelay: time.Millisecond, Multiplier: 2.0}
	res := Do(p, func() error { return nil })
	if res.Err != nil || res.Attempts != 1 {
		t.Fatal("首次成功")
	}
}

func TestDo_RetryThenSuccess(t *testing.T) {
	p := Policy{MaxAttempts: 5, BaseDelay: time.Millisecond, Multiplier: 2.0}
	calls := 0
	res := Do(p, func() error {
		calls++
		if calls < 3 {
			return errors.New("tmp")
		}
		return nil
	})
	if res.Err != nil || res.Attempts != 3 {
		t.Fatal("应 3 次成功")
	}
}

func TestDo_MaxAttempts(t *testing.T) {
	p := Policy{MaxAttempts: 3, BaseDelay: time.Millisecond, Multiplier: 2.0}
	res := Do(p, func() error { return errors.New("always") })
	if res.Attempts != 3 {
		t.Fatal("attempts")
	}
	if !errors.Is(res.Err, ErrMaxAttempts) {
		t.Fatal("应 ErrMaxAttempts")
	}
}

func TestDo_Permanent(t *testing.T) {
	p := Policy{MaxAttempts: 5, BaseDelay: time.Millisecond, Multiplier: 2.0}
	calls := 0
	res := Do(p, func() error {
		calls++
		return Permanent(errors.New("fatal"))
	})
	if calls != 1 {
		t.Fatal("不应重试永久错误")
	}
	if !errors.Is(res.Err, ErrMaxAttempts) == false {
		t.Fatal("err 应是 wrapped")
	}
}

func TestDelayFor(t *testing.T) {
	p := Policy{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second, Multiplier: 2.0, Jitter: false}
	for i := 1; i <= 5; i++ {
		d := delayFor(p, i)
		if d <= 0 {
			t.Fatal("delay")
		}
	}
}

func TestDefault(t *testing.T) {
	p := Default()
	if p.MaxAttempts <= 0 {
		t.Fatal("default")
	}
}
