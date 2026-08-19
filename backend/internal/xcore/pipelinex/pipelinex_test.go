package pipelinex

import (
	"context"
	"errors"
	"testing"
)

func TestRun(t *testing.T) {
	p := New[int]()
	p.Add(func(_ context.Context, in int) (int, error) { return in + 1, nil })
	p.Add(func(_ context.Context, in int) (int, error) { return in * 2, nil })
	v, err := p.Run(context.Background(), 3)
	if err != nil || v != 8 {
		t.Fatal("run", v)
	}
}

func TestRunErr(t *testing.T) {
	p := New[int]()
	myErr := errors.New("bad")
	p.Add(func(_ context.Context, _ int) (int, error) { return 0, myErr })
	if _, err := p.Run(context.Background(), 0); err == nil {
		t.Fatal("err")
	}
}

func TestRunCtxCancel(t *testing.T) {
	p := New[int]()
	p.Add(func(ctx context.Context, in int) (int, error) { return in, ctx.Err() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Run(ctx, 0); err == nil {
		t.Fatal("ctx")
	}
}

func TestLen(t *testing.T) {
	p := New[int]()
	if p.Len() != 0 {
		t.Fatal("empty")
	}
	p.Add(func(_ context.Context, in int) (int, error) { return in, nil })
	if p.Len() != 1 {
		t.Fatal("len")
	}
}
