package wpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSubmit_Run(t *testing.T) {
	p := New(2, 8)
	defer p.Close()
	var ran atomic.Uint64
	if err := p.Submit(Task{Tenant: "t1", Run: func(ctx context.Context) error {
		ran.Add(1)
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ran.Load() == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("did not run")
}

func TestSubmit_DefaultTenant(t *testing.T) {
	p := New(1, 8)
	defer p.Close()
	var ran atomic.Uint64
	if err := p.Submit(Task{Run: func(ctx context.Context) error {
		ran.Add(1)
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if ran.Load() != 1 {
		t.Fatal("default tenant")
	}
}

func TestSubmit_Closed(t *testing.T) {
	p := New(1, 8)
	p.Close()
	if err := p.Submit(Task{Run: func(ctx context.Context) error { return nil }}); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestFairness(t *testing.T) {
	p := New(1, 8)
	defer p.Close()
	var order []string
	var mu sync.Mutex
	for _, tnt := range []string{"a", "b", "c"} {
		t := tnt
		for i := 0; i < 2; i++ {
			p.Submit(Task{Tenant: t, Run: func(ctx context.Context) error {
				mu.Lock()
				order = append(order, t)
				mu.Unlock()
				return nil
			}})
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := len(order)
		mu.Unlock()
		if done == 6 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 6 {
		t.Fatal("did not run all")
	}
}

func TestStats(t *testing.T) {
	p := New(2, 8)
	defer p.Close()
	p.Submit(Task{Tenant: "a", Run: func(ctx context.Context) error { return nil }})
	time.Sleep(50 * time.Millisecond)
	s := p.Stats()
	if s.Workers != 2 || s.Tenants < 2 {
		t.Fatal("stats")
	}
}
