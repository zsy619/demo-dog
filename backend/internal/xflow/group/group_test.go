package group

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestOK(t *testing.T) {
	g := New(context.Background())
	for i := 0; i < 5; i++ {
		g.Go(func(_ context.Context) error {
			time.Sleep(5 * time.Millisecond)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestError(t *testing.T) {
	g := New(context.Background())
	myErr := errors.New("x")
	g.Go(func(_ context.Context) error { return myErr })
	if err := g.Wait(); !errors.Is(err, myErr) {
		t.Fatal("err")
	}
}

func TestMultiError(t *testing.T) {
	g := New(context.Background())
	g.Go(func(_ context.Context) error { return errors.New("a") })
	g.Go(func(_ context.Context) error { return errors.New("b") })
	err := g.Wait()
	if me, ok := err.(*MultiError); !ok || len(me.Errs) != 2 {
		t.Fatal("multi", err)
	}
}

func TestLimitConcurrent(t *testing.T) {
	g := New(context.Background()).SetLimit(2)
	var running atomic.Int32
	var max atomic.Int32
	for i := 0; i < 10; i++ {
		g.Go(func(_ context.Context) error {
			r := running.Add(1)
			for {
				m := max.Load()
				if r <= m || max.CompareAndSwap(m, r) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			running.Add(-1)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if max.Load() > 2 {
		t.Fatal("limit", max.Load())
	}
}

func TestPanic(t *testing.T) {
	g := New(context.Background())
	g.Go(func(_ context.Context) error { panic("boom") })
	err := g.Wait()
	if err == nil {
		t.Fatal("panic 应转为 error")
	}
}

func TestStats(t *testing.T) {
	g := New(context.Background())
	for i := 0; i < 5; i++ {
		i := i
		g.Go(func(_ context.Context) error {
			if i == 2 {
				return errors.New("err")
			}
			return nil
		})
	}
	g.Wait()
	st := g.Stats()
	if st.GoCount != 5 {
		t.Fatal("GoCount", st.GoCount)
	}
	if st.DoneCount != 5 {
		t.Fatal("DoneCount", st.DoneCount)
	}
	if st.ErrorCount < 1 {
		t.Fatal("ErrorCount", st.ErrorCount)
	}
}

func TestMultiErrorUnwrap(t *testing.T) {
	g := New(context.Background())
	myErr := errors.New("a")
	g.Go(func(_ context.Context) error { return myErr })
	g.Go(func(_ context.Context) error { return errors.New("b") })
	err := g.Wait()
	if !errors.Is(err, myErr) {
		t.Fatal("MultiError 应可 errors.Is")
	}
	var m *MultiError
	if !errors.As(err, &m) {
		t.Fatal("应可 errors.As")
	}
}

func TestNilCtx(t *testing.T) {
	g := New(nil)
	g.Go(func(ctx context.Context) error { return nil })
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestCancel(t *testing.T) {
	g := New(context.Background())
	g.Go(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	time.Sleep(5 * time.Millisecond)
	g.Cancel()
	if err := g.Wait(); err == nil {
		t.Fatal("应返回 ctx 错误")
	}
}
