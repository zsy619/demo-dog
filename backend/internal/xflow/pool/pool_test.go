package pool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_BasicJob(t *testing.T) {
	p := New(Config{Workers: 2, QueueCap: 4, Name: "t"})
	p.Start()
	defer p.Stop()
	var ran atomic.Uint64
	for i := 0; i < 4; i++ {
		if err := p.Submit(Job{Name: "j", Run: func(ctx context.Context) error {
			ran.Add(1)
			return nil
		}}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.After(time.Second)
	for ran.Load() < 4 {
		select {
		case <-deadline:
			t.Fatal("timeout")
		case <-time.After(10 * time.Millisecond):
		}
	}
	s := p.Stats()
	if s.Completed != 4 {
		t.Fatalf("completed: %d", s.Completed)
	}
}

func TestPool_ErrorCounted(t *testing.T) {
	p := New(Config{Workers: 2, QueueCap: 4, Name: "t"})
	p.Start()
	defer p.Stop()
	for i := 0; i < 4; i++ {
		if err := p.Submit(Job{Name: "j", Run: func(ctx context.Context) error {
			return errors.New("x")
		}}); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(200 * time.Millisecond)
	s := p.Stats()
	if s.Failed != 4 {
		t.Fatalf("failed: %d", s.Failed)
	}
}

func TestPool_PanicRecovered(t *testing.T) {
	panics := atomic.Uint64{}
	p := New(Config{Workers: 1, QueueCap: 1, Name: "t", OnPanic: func(name string, r any) {
		panics.Add(1)
	}})
	p.Start()
	defer p.Stop()
	if err := p.Submit(Job{Name: "p", Run: func(ctx context.Context) error {
		panic("boom")
	}}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	s := p.Stats()
	if s.Panicked != 1 {
		t.Fatalf("panicked: %d", s.Panicked)
	}
	if panics.Load() != 1 {
		t.Fatal("OnPanic not called")
	}
}

func TestPool_Backpressure(t *testing.T) {
	p := New(Config{Workers: 1, QueueCap: 1, Name: "t"})
	p.Start()
	defer p.Stop()
	// Fill queue + slot with a slow job.
	if err := p.Submit(Job{Name: "slow", Run: func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	// Try to enqueue many fast jobs; at least some should be dropped.
	dropped := 0
	for i := 0; i < 20; i++ {
		if err := p.Submit(Job{Name: "overflow", Run: func(ctx context.Context) error {
			return nil
		}}); err != nil {
			dropped++
		}
	}
	if dropped == 0 {
		t.Fatal("expected some dropped")
	}
	s := p.Stats()
	if s.Dropped == 0 {
		t.Fatal("Stats.Dropped zero")
	}
}

func TestPool_NotRunning(t *testing.T) {
	p := New(Config{Workers: 1, QueueCap: 1, Name: "t"})
	if err := p.Submit(Job{Name: "j", Run: func(ctx context.Context) error { return nil }}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPool_DoubleStartSafe(t *testing.T) {
	p := New(Config{Workers: 1, QueueCap: 1, Name: "t"})
	p.Start()
	p.Start()
	defer p.Stop()
	s := p.Stats()
	if s.Workers != 1 {
		t.Fatal("double start spawned extras")
	}
}

func TestPool_SubmitCtxCancel(t *testing.T) {
	p := New(Config{Workers: 1, QueueCap: 1, Name: "t"})
	p.Start()
	defer p.Stop()
	for i := 0; i < 2; i++ {
		p.Submit(Job{Name: "slow", Run: func(ctx context.Context) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		}})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.SubmitCtx(ctx, Job{Name: "x", Run: func(ctx context.Context) error { return nil }}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPool_ResultsBuffered(t *testing.T) {
	p := New(Config{Workers: 1, QueueCap: 8, Name: "t"})
	p.Start()
	for i := 0; i < 4; i++ {
		if err := p.Submit(Job{Name: "j", Run: func(ctx context.Context) error {
			return errors.New("x")
		}}); err != nil {
			t.Fatal(err)
		}
	}
	got := 0
	timeout := time.After(time.Second)
loop:
	for {
		select {
		case r, ok := <-p.Results():
			if !ok {
				break loop
			}
			if r.Err == nil {
				t.Errorf("expected error")
			}
			got++
		case <-timeout:
			break loop
		}
	}
	p.Stop()
	if got < 1 {
		t.Fatalf("no results: %d", got)
	}
}
