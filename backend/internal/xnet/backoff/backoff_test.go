package backoff

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNext_Exponential(t *testing.T) {
	s := Strategy{Base: 10 * time.Millisecond, Max: 1 * time.Second}
	if s.Next(0) < 5*time.Millisecond {
		t.Fatal("first")
	}
}

func TestNext_Cap(t *testing.T) {
	s := Strategy{Base: 1 * time.Second, Max: 2 * time.Second}
	if s.Next(10) > 3*time.Second {
		t.Fatal("cap")
	}
}

func TestDo_Success(t *testing.T) {
	s := Default()
	calls := 0
	err := s.Do(context.Background(), func() error {
		calls++
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatal("success")
	}
}

func TestDo_RetryThenSuccess(t *testing.T) {
	s := Strategy{Base: 5 * time.Millisecond, Max: 10 * time.Millisecond, Attempts: 3}
	calls := 0
	err := s.Do(context.Background(), func() error {
		calls++
		if calls < 3 {
			return errors.New("fail")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatal("retry")
	}
}

func TestDo_AllFail(t *testing.T) {
	s := Strategy{Base: 1 * time.Millisecond, Attempts: 2}
	err := s.Do(context.Background(), func() error { return errors.New("fail") })
	if err == nil {
		t.Fatal("应失败")
	}
}

func TestDo_Cancel(t *testing.T) {
	s := Strategy{Base: 100 * time.Millisecond, Attempts: 10}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := s.Do(ctx, func() error { return errors.New("fail") })
	if err == nil || time.Since(start) > 500*time.Millisecond {
		t.Fatal("cancel")
	}
}
