package topo

import (
	"errors"
	"testing"
)

func TestAdd_RootIsReady(t *testing.T) {
	q := New()
	q.Add("a", "")
	id, ok := q.Next()
	if !ok || id != "a" {
		t.Fatal("root should be ready")
	}
}

func TestAdd_ChildWaitsForParent(t *testing.T) {
	q := New()
	q.Add("a", "")
	q.Add("b", "a")
	// Drain ready (a).
	if id, _ := q.Next(); id != "a" {
		t.Fatal("a ready")
	}
	// b should not be ready until a completes.
	select {
	case id := <-q.ready:
		t.Fatalf("b should not be ready, got %s", id)
	default:
	}
}

func TestComplete_UnblocksChild(t *testing.T) {
	q := New()
	q.Add("a", "")
	q.Add("b", "a")
	q.Next()
	q.Start("a")
	q.Complete("a", nil)
	id, ok := q.Next()
	if !ok || id != "b" {
		t.Fatal("b should be ready after a done")
	}
}

func TestComplete_Failed(t *testing.T) {
	q := New()
	q.Add("a", "")
	q.Add("b", "a")
	q.Next()
	q.Start("a")
	q.Complete("a", errors.New("oops"))
	// b should not be ready.
	select {
	case id := <-q.ready:
		t.Fatalf("b should not be ready after a failed, got %s", id)
	default:
	}
}

func TestStart_NotPending(t *testing.T) {
	q := New()
	q.Add("a", "")
	q.Next()
	q.Start("a")
	if err := q.Start("a"); err == nil {
		t.Fatal("expected error")
	}
}

func TestStart_Missing(t *testing.T) {
	q := New()
	if err := q.Start("missing"); err != ErrNoTask {
		t.Fatal("expected ErrNoTask")
	}
}

func TestComplete_Missing(t *testing.T) {
	q := New()
	if err := q.Complete("missing", nil); err != ErrNoTask {
		t.Fatal("expected ErrNoTask")
	}
}

func TestGet(t *testing.T) {
	q := New()
	q.Add("a", "")
	task, ok := q.Get("a")
	if !ok || task.ID != "a" {
		t.Fatal("get")
	}
	if _, ok := q.Get("missing"); ok {
		t.Fatal("missing")
	}
}

func TestStats(t *testing.T) {
	q := New()
	q.Add("a", "")
	q.Add("b", "")
	q.Add("c", "a")
	q.Next()
	q.Start("a")
	q.Complete("a", nil)
	s := q.Stats()
	if s.Tasks != 3 || s.Done != 1 {
		t.Fatalf("stats: %+v", s)
	}
}

func TestClose(t *testing.T) {
	q := New()
	q.Close()
	if _, ok := q.Next(); ok {
		t.Fatal("closed should not yield")
	}
}

func TestStatusString(t *testing.T) {
	if StatusPending.String() != "pending" {
		t.Fatal("pending")
	}
	if StatusRunning.String() != "running" {
		t.Fatal("running")
	}
	if StatusDone.String() != "done" {
		t.Fatal("done")
	}
	if StatusFailed.String() != "failed" {
		t.Fatal("failed")
	}
}

func TestAdd_Duplicate(t *testing.T) {
	q := New()
	q.Add("a", "")
	q.Add("a", "")
	if q.Stats().Tasks != 1 {
		t.Fatal("dup")
	}
}
