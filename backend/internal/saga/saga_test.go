package saga

import (
	"context"
	"errors"
	"testing"
)

func TestRun_AllSuccess(t *testing.T) {
	c := New()
	steps := []Step{
		{Name: "a", Do: func(ctx context.Context) error { return nil }, Undo: func(ctx context.Context) error { return nil }},
		{Name: "b", Do: func(ctx context.Context) error { return nil }, Undo: func(ctx context.Context) error { return nil }},
	}
	o := c.Run(context.Background(), steps)
	if o.Error != nil || o.Rollback {
		t.Fatalf("outcome: %+v", o)
	}
	if len(o.Executed) != 2 {
		t.Fatal("executed")
	}
	if len(o.Undone) != 0 {
		t.Fatal("undone")
	}
}

func TestRun_StepFails(t *testing.T) {
	c := New()
	steps := []Step{
		{Name: "a", Do: func(ctx context.Context) error { return nil }, Undo: func(ctx context.Context) error {
			t.Log("undo a")
			return nil
		}},
		{Name: "b", Do: func(ctx context.Context) error { return errors.New("boom") }, Undo: func(ctx context.Context) error { return nil }},
		{Name: "c", Do: func(ctx context.Context) error { return nil }, Undo: func(ctx context.Context) error { return nil }},
	}
	o := c.Run(context.Background(), steps)
	if !o.Rollback || o.Error == nil {
		t.Fatal("expected rollback")
	}
	if o.Name != "b" {
		t.Fatal("name")
	}
	if len(o.Undone) != 1 || o.Undone[0] != "a" {
		t.Fatalf("undone: %v", o.Undone)
	}
}

func TestRun_Empty(t *testing.T) {
	c := New()
	o := c.Run(context.Background(), nil)
	if o.Rollback || o.Error != nil {
		t.Fatal("empty should be no-op")
	}
}

func TestRun_OnRollbackHook(t *testing.T) {
	c := New()
	var got Outcome
	c.OnRollback(func(o Outcome) {
		got = o
	})
	steps := []Step{
		{Name: "a", Do: func(ctx context.Context) error { return nil }, Undo: func(ctx context.Context) error { return nil }},
		{Name: "b", Do: func(ctx context.Context) error { return errors.New("bad") }, Undo: func(ctx context.Context) error { return nil }},
	}
	c.Run(context.Background(), steps)
	if !got.Rollback {
		t.Fatal("hook not called")
	}
}

func TestRun_UndoErrors(t *testing.T) {
	c := New()
	steps := []Step{
		{Name: "a", Do: func(ctx context.Context) error { return nil }, Undo: func(ctx context.Context) error { return errors.New("undo-fail") }},
		{Name: "b", Do: func(ctx context.Context) error { return errors.New("boom") }, Undo: func(ctx context.Context) error { return nil }},
	}
	o := c.Run(context.Background(), steps)
	if !o.Rollback {
		t.Fatal("rollback")
	}
}

func TestRun_NoUndoFunction(t *testing.T) {
	c := New()
	steps := []Step{
		{Name: "a", Do: func(ctx context.Context) error { return nil }},
		{Name: "b", Do: func(ctx context.Context) error { return errors.New("x") }},
	}
	o := c.Run(context.Background(), steps)
	if !o.Rollback || len(o.Undone) != 0 {
		t.Fatal("no undo should not error")
	}
}

func TestRun_ContextCancel(t *testing.T) {
	c := New()
	steps := []Step{
		{Name: "a", Do: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}, Undo: func(ctx context.Context) error { return nil }},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o := c.Run(ctx, steps)
	if !o.Rollback {
		t.Fatal("should rollback")
	}
}
