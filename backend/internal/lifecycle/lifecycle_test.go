package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMakeStopper(t *testing.T) {
	s := MakeStopper("x", func(ctx context.Context) error { return nil })
	if s.Name() != "x" {
		t.Fatal("name")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCoordinator_Empty(t *testing.T) {
	c := NewCoordinator()
	if err := c.Shutdown(time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestCoordinator_Ordered(t *testing.T) {
	c := NewCoordinator()
	order := []string{}
	for _, name := range []string{"a", "b", "c"} {
		name := name
		c.Register(MakeStopper(name, func(ctx context.Context) error {
			order = append(order, name)
			return nil
		}))
	}
	if err := c.Shutdown(time.Second); err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 || order[0] != "a" || order[2] != "c" {
		t.Fatalf("order: %v", order)
	}
}

func TestCoordinator_ErrorPropagates(t *testing.T) {
	c := NewCoordinator()
	c.Register(MakeStopper("a", func(ctx context.Context) error { return nil }))
	c.Register(MakeStopper("b", func(ctx context.Context) error { return errors.New("boom") }))
	c.Register(MakeStopper("c", func(ctx context.Context) error { return nil }))
	if err := c.Shutdown(time.Second); err == nil {
		t.Fatal("expected error")
	}
	prog := c.Progresses()
	if len(prog) != 3 {
		t.Fatalf("progress: %d", len(prog))
	}
	if prog[1].Status != "failed" {
		t.Fatal(prog[1])
	}
	if prog[0].Status != "stopped" {
		t.Fatal(prog[0])
	}
}

func TestCoordinator_TimesOut(t *testing.T) {
	c := NewCoordinator()
	c.Register(MakeStopper("slow", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Hour):
			return nil
		}
	}))
	err := c.Shutdown(50 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	prog := c.Progresses()
	if prog[0].Status != "timed_out" && prog[0].Status != "failed" {
		t.Fatalf("status: %s", prog[0].Status)
	}
}

func TestCoordinator_DoubleShutdownFails(t *testing.T) {
	c := NewCoordinator()
	c.Register(MakeStopper("a", func(ctx context.Context) error { return nil }))
	if err := c.Shutdown(time.Second); err != nil {
		t.Fatal(err)
	}
	if err := c.Shutdown(time.Second); err == nil {
		t.Fatal("expected double-shutdown error")
	}
}

func TestCoordinator_ClosedAndClosedAt(t *testing.T) {
	c := NewCoordinator()
	if c.Closed() {
		t.Fatal("should not be closed")
	}
	c.Shutdown(time.Second)
	if !c.Closed() {
		t.Fatal("should be closed")
	}
	if c.ClosedAt().IsZero() {
		t.Fatal("closed at zero")
	}
}

func TestCoordinator_ContinuesAfterError(t *testing.T) {
	c := NewCoordinator()
	ran := false
	c.Register(MakeStopper("a", func(ctx context.Context) error {
		ran = true
		return errors.New("bad")
	}))
	c.Register(MakeStopper("b", func(ctx context.Context) error {
		if !ran {
			t.Fatal("b ran before a")
		}
		return nil
	}))
	c.Shutdown(time.Second)
}

func TestJoinErr(t *testing.T) {
	e := joinErr([]error{errors.New("x"), errors.New("y")})
	if e.Error() != "x; y" {
		t.Fatal(e)
	}
}
