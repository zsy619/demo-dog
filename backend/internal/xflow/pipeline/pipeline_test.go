package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	p := New[string]()
	p.Append(func(_ context.Context, v string) (string, error) { return v + "a", nil })
	p.Append(func(_ context.Context, v string) (string, error) { return v + "b", nil })
	p.Append(func(_ context.Context, v string) (string, error) { return strings.ToUpper(v), nil })
	out, err := p.Run(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if out != "HIAB" {
		t.Fatal("out", out)
	}
}

func TestRun_Error(t *testing.T) {
	p := New[int]()
	myErr := errors.New("bad")
	p.Append(func(_ context.Context, v int) (int, error) { return v + 1, nil })
	p.Append(func(_ context.Context, v int) (int, error) { return v, myErr })
	p.Append(func(_ context.Context, v int) (int, error) { return v * 10, nil })
	out, err := p.Run(context.Background(), 1)
	if !errors.Is(err, myErr) || out != 2 {
		t.Fatal("err")
	}
}

func TestLen(t *testing.T) {
	p := New[int]()
	if p.Len() != 0 {
		t.Fatal("empty")
	}
	p.Append(func(_ context.Context, v int) (int, error) { return v, nil })
	if p.Len() != 1 {
		t.Fatal("len")
	}
}

func TestRun_Empty(t *testing.T) {
	p := New[int]()
	v, err := p.Run(context.Background(), 7)
	if err != nil || v != 7 {
		t.Fatal("empty")
	}
}
