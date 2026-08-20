package tpc

import (
	"context"
	"errors"
	"testing"
)

func TestRun_AllSuccess(t *testing.T) {
	c := New()
	res := c.Run(context.Background(), []Resource{
		{Name: "a", Prepare: func(ctx context.Context) error { return nil }, Commit: func(ctx context.Context) error { return nil }},
		{Name: "b", Prepare: func(ctx context.Context) error { return nil }, Commit: func(ctx context.Context) error { return nil }},
	})
	if res.Phase != PhaseCommit || res.Error != nil {
		t.Fatal(res)
	}
	if len(res.Committed) != 2 {
		t.Fatal("committed")
	}
	if len(res.Aborted) != 0 {
		t.Fatal("aborted should be empty")
	}
}

func TestRun_PrepareFails(t *testing.T) {
	c := New()
	res := c.Run(context.Background(), []Resource{
		{Name: "a", Prepare: func(ctx context.Context) error { return nil }, Commit: func(ctx context.Context) error { return nil }, Abort: func(ctx context.Context) error { return nil }},
		{Name: "b", Prepare: func(ctx context.Context) error { return errors.New("prep") }, Abort: func(ctx context.Context) error { return nil }},
	})
	if res.Phase != PhaseAbort {
		t.Fatal("should abort")
	}
	if len(res.Aborted) != 1 || res.Aborted[0] != "a" {
		t.Fatalf("aborted: %v", res.Aborted)
	}
}

func TestRun_Empty(t *testing.T) {
	c := New()
	res := c.Run(context.Background(), nil)
	if res.Error == nil {
		t.Fatal("expected error")
	}
}

func TestRun_AbortErrorIgnored(t *testing.T) {
	c := New()
	res := c.Run(context.Background(), []Resource{
		{Name: "a", Prepare: func(ctx context.Context) error { return nil }, Abort: func(ctx context.Context) error { return errors.New("abort-fail") }},
		{Name: "b", Prepare: func(ctx context.Context) error { return errors.New("no") }, Abort: func(ctx context.Context) error { return nil }},
	})
	if res.Phase != PhaseAbort {
		t.Fatal("abort")
	}
	if len(res.Aborted) != 1 {
		t.Fatal("aborted")
	}
}

func TestRun_AbortNilSkipped(t *testing.T) {
	c := New()
	res := c.Run(context.Background(), []Resource{
		{Name: "a", Prepare: func(ctx context.Context) error { return nil }},
		{Name: "b", Prepare: func(ctx context.Context) error { return errors.New("x") }},
	})
	if res.Phase != PhaseAbort {
		t.Fatal("abort")
	}
	if len(res.Aborted) != 0 {
		t.Fatal("nil abort should not be called")
	}
}

func TestRun_CommitError(t *testing.T) {
	c := New()
	res := c.Run(context.Background(), []Resource{
		{Name: "a", Prepare: func(ctx context.Context) error { return nil }, Commit: func(ctx context.Context) error { return errors.New("commit fail") }},
	})
	if res.Phase != PhaseAbort || res.Error == nil {
		t.Fatal("commit failure should abort")
	}
}

func TestRun_ContextCancel(t *testing.T) {
	c := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := c.Run(ctx, []Resource{
		{Name: "a", Prepare: func(ctx context.Context) error { return ctx.Err() }, Abort: func(ctx context.Context) error { return nil }},
	})
	if res.Phase != PhaseAbort {
		t.Fatal("ctx cancel should abort")
	}
}

func TestRun_PreparedOrder(t *testing.T) {
	c := New()
	var called []string
	for _, name := range []string{"a", "b", "c"} {
		n := name
		defer func() {
			// capture in closure for stable iteration
		}()
		_ = n
	}
	res := c.Run(context.Background(), []Resource{
		{Name: "a", Prepare: func(ctx context.Context) error { called = append(called, "a-prep"); return nil }, Commit: func(ctx context.Context) error { return nil }},
		{Name: "b", Prepare: func(ctx context.Context) error { called = append(called, "b-prep"); return nil }, Commit: func(ctx context.Context) error { return nil }},
		{Name: "c", Prepare: func(ctx context.Context) error { called = append(called, "c-prep"); return nil }, Commit: func(ctx context.Context) error { return nil }},
	})
	if res.Phase != PhaseCommit {
		t.Fatal("phase")
	}
	if len(called) != 3 {
		t.Fatal("calls")
	}
}
