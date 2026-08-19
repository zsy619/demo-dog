package probe

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunOnce(t *testing.T) {
	p := New(Config{Interval: 50 * time.Millisecond}, func(_ context.Context, _ string) error { return nil },
		[]Target{{Name: "a", Addr: "127.0.0.1"}})
	if err := p.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	r, ok := p.ResultOf("a")
	if !ok || r.Status != StatusHealthy {
		t.Fatal("应为 Healthy")
	}
}

func TestRunOnce_Fail(t *testing.T) {
	p := New(Config{}, func(_ context.Context, _ string) error { return errors.New("boom") },
		[]Target{{Name: "a", Addr: "x"}})
	p.RunOnce(context.Background())
	r, _ := p.ResultOf("a")
	if r.Status != StatusUnhealthy {
		t.Fatal("应 Unhealthy")
	}
	if r.Error == "" {
		t.Fatal("应有错误信息")
	}
}

func TestRunOnce_Empty(t *testing.T) {
	p := New(Config{}, nil, nil)
	if err := p.RunOnce(context.Background()); !errors.Is(err, ErrEmptyTargets) {
		t.Fatal("应 ErrEmptyTargets")
	}
}

func TestStartStop(t *testing.T) {
	p := New(Config{Interval: 30 * time.Millisecond}, nil, []Target{{Name: "a", Addr: "x"}})
	p.Start()
	time.Sleep(100 * time.Millisecond)
	p.Stop()
	if p.Rounds() == 0 {
		t.Fatal("应至少执行一轮")
	}
}

func TestSnapshot(t *testing.T) {
	p := New(Config{}, nil, []Target{{Name: "a"}, {Name: "b"}})
	p.RunOnce(context.Background())
	snap := p.Snapshot()
	if len(snap) != 2 {
		t.Fatal("snapshot")
	}
}

func TestDoubleStart(t *testing.T) {
	p := New(Config{Interval: 50 * time.Millisecond}, nil, []Target{{Name: "a"}})
	p.Start()
	p.Start()
	p.Stop()
}
