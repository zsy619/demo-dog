package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCheck_AllOK(t *testing.T) {
	m := New()
	m.Register(CheckerFunc{N: "a", F: func(_ context.Context) error { return nil }})
	m.Register(CheckerFunc{N: "b", F: func(_ context.Context) error { return nil }})
	r := m.Check(context.Background())
	if len(r) != 2 || Overall(r) != StatusOK {
		t.Fatal("ok")
	}
}

func TestCheck_Down(t *testing.T) {
	m := New()
	m.Register(CheckerFunc{N: "a", F: func(_ context.Context) error { return nil }})
	m.Register(CheckerFunc{N: "b", F: func(_ context.Context) error { return errors.New("x") }})
	r := m.Check(context.Background())
	if Overall(r) != StatusDown {
		t.Fatal("down")
	}
}

func TestStatusString(t *testing.T) {
	if StatusOK.String() != "ok" {
		t.Fatal("str")
	}
	if StatusDown.String() != "down" {
		t.Fatal("str")
	}
}

func TestCheck_Slow(t *testing.T) {
	m := New()
	m.Register(CheckerFunc{N: "slow", F: func(_ context.Context) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}})
	start := time.Now()
	r := m.Check(context.Background())
	if r[0].Latency < 20*time.Millisecond {
		t.Fatal("latency")
	}
	_ = start
}
