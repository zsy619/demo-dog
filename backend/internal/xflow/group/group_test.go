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

func TestLimit(t *testing.T) {
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
