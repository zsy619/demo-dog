package proxy

import (
	"context"
	"errors"
	"testing"
)

func bs() []*Backend {
	return []*Backend{
		{Name: "a", URL: "http://a"},
		{Name: "b", URL: "http://b"},
		{Name: "c", URL: "http://c"},
	}
}

func TestPick_Default(t *testing.T) {
	p := New(bs())
	b, err := p.Pick()
	if err != nil || b == nil {
		t.Fatal("pick")
	}
}

func TestPick_RoundRobin(t *testing.T) {
	p := New(bs())
	b1, _ := p.Pick()
	b2, _ := p.Pick()
	b3, _ := p.Pick()
	b4, _ := p.Pick()
	if b1.Name != "a" || b2.Name != "b" || b3.Name != "c" || b4.Name != "a" {
		t.Fatalf("rr: %s %s %s %s", b1.Name, b2.Name, b3.Name, b4.Name)
	}
}

func TestPick_NoBackends(t *testing.T) {
	p := New(nil)
	if _, err := p.Pick(); !errors.Is(err, ErrNoBackend) {
		t.Fatal(err)
	}
}

func TestPick_AllDead(t *testing.T) {
	p := New(bs())
	p.MarkDown("a")
	p.MarkDown("b")
	p.MarkDown("c")
	if _, err := p.Pick(); !errors.Is(err, ErrNoBackend) {
		t.Fatal(err)
	}
}

func TestMarkUpDown(t *testing.T) {
	p := New(bs())
	p.MarkDown("a")
	b, _ := p.Pick()
	if b.Name != "b" {
		t.Fatal("should pick b")
	}
	p.MarkUp("a")
	cnt := 0
	for i := 0; i < 3; i++ {
		b, _ = p.Pick()
		if b.Name == "a" {
			cnt++
		}
	}
	if cnt == 0 {
		t.Fatal("a should appear after MarkUp")
	}
}

func TestBackends(t *testing.T) {
	p := New(bs())
	views := p.Backends()
	if len(views) != 3 || views[0].Name != "a" {
		t.Fatal("views")
	}
}

func TestStats(t *testing.T) {
	p := New(bs())
	p.Pick()
	p.Pick()
	s := p.Stats()
	if s.Picks != 2 {
		t.Fatal("picks")
	}
}

func TestDoWithFallback(t *testing.T) {
	p := New(bs())
	err := p.DoWithFallback(context.Background(), func(ctx context.Context, b *Backend) error {
		if b.Name == "a" {
			return errors.New("a fail")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDoWithFallback_AllFail(t *testing.T) {
	p := New(bs())
	err := p.DoWithFallback(context.Background(), func(ctx context.Context, b *Backend) error {
		return errors.New("fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDoWithFallback_NoBackends(t *testing.T) {
	p := New(nil)
	if err := p.DoWithFallback(context.Background(), func(ctx context.Context, b *Backend) error {
		return nil
	}); !errors.Is(err, ErrNoBackend) {
		t.Fatal(err)
	}
}

func TestDoWithFallback_AllDead(t *testing.T) {
	p := New(bs())
	p.MarkDown("a")
	p.MarkDown("b")
	p.MarkDown("c")
	if err := p.DoWithFallback(context.Background(), func(ctx context.Context, b *Backend) error {
		return nil
	}); !errors.Is(err, ErrNoBackend) {
		t.Fatal(err)
	}
}

func TestDoWithFallback_ContextCancel(t *testing.T) {
	p := New(bs())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.DoWithFallback(ctx, func(ctx context.Context, b *Backend) error {
		return nil
	}); err == nil {
		t.Fatal("expected ctx cancel")
	}
}
