package keepalive

import (
	"context"
	"errors"
	"testing"
	"time"
)

type okPing struct{ n int }

func (p *okPing) Ping() error { p.n++; return nil }

type errPing struct{ n int }

func (p *errPing) Ping() error { p.n++; return errors.New("ping fail") }

func TestStartStop(t *testing.T) {
	p := &okPing{}
	m := New(p, 10*time.Millisecond, 50*time.Millisecond)
	m.Start(context.Background())
	time.Sleep(60 * time.Millisecond)
	m.Stop()
	if m.Successes() == 0 {
		t.Fatal("ok not called")
	}
}

func TestErr(t *testing.T) {
	p := &errPing{}
	m := New(p, 10*time.Millisecond, 50*time.Millisecond)
	m.Start(context.Background())
	time.Sleep(60 * time.Millisecond)
	m.Stop()
	if m.Misses() == 0 {
		t.Fatal("miss")
	}
}

func TestIdempotent(t *testing.T) {
	p := &okPing{}
	m := New(p, 100*time.Millisecond, 100*time.Millisecond)
	m.Start(context.Background())
	m.Start(context.Background()) // 重复 start 应幂等
	time.Sleep(20 * time.Millisecond)
	m.Stop()
}

func TestStopIdempotent(t *testing.T) {
	p := &okPing{}
	m := New(p, 100*time.Millisecond, 100*time.Millisecond)
	m.Start(context.Background())
	time.Sleep(20 * time.Millisecond)
	m.Stop()
	m.Stop() // 不应 panic
}
