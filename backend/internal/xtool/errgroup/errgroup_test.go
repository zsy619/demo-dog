package errgroup

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOK(t *testing.T) {
	g := New()
	g.Go(func() error { return nil })
	g.Go(func() error { return nil })
	if err := g.Wait(); err != nil {
		t.Fatal("ok", err)
	}
}

func TestErr(t *testing.T) {
	myErr := errors.New("x")
	g := New()
	g.Go(func() error { return myErr })
	g.Go(func() error { return nil })
	if err := g.Wait(); err != myErr {
		t.Fatal("err", err)
	}
}

func TestWithContext(t *testing.T) {
	g, ctx := WithContext(context.Background())
	done := make(chan struct{})
	g.Go(func() error {
		<-ctx.Done()
		close(done)
		return ctx.Err()
	})
	time.Sleep(10 * time.Millisecond)
	g.Go(func() error { return errors.New("stop") })
	g.Wait()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ctx 未取消")
	}
}
