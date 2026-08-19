package migrate

import (
	"context"
	"errors"
	"testing"
)

func TestApply(t *testing.T) {
	store := NewMemStore()
	list := []Migration{
		{Name: "a", Up: func(_ context.Context, _ *Migration) error { return nil }},
		{Name: "b", Up: func(_ context.Context, _ *Migration) error { return nil }},
	}
	m := New(store, list)
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.Applied() != 2 || m.Pending() != 0 {
		t.Fatal("应用计数错")
	}
}

func TestApply_Idempotent(t *testing.T) {
	store := NewMemStore()
	list := []Migration{{Name: "a", Up: func(_ context.Context, _ *Migration) error { return nil }}}
	m := New(store, list)
	m.Apply(context.Background())
	m.Apply(context.Background())
	if m.Applied() != 1 {
		t.Fatal("重复 Apply 不应执行")
	}
}

func TestApply_Fail(t *testing.T) {
	store := NewMemStore()
	list := []Migration{{Name: "a", Up: func(_ context.Context, _ *Migration) error { return errors.New("x") }}}
	m := New(store, list)
	if err := m.Apply(context.Background()); err == nil {
		t.Fatal("应报错")
	}
}

func TestRollback(t *testing.T) {
	store := NewMemStore()
	called := false
	list := []Migration{{
		Name: "a",
		Up:   func(_ context.Context, _ *Migration) error { return nil },
		Down: func(_ context.Context, _ *Migration) error { called = true; return nil },
	}}
	m := New(store, list)
	m.Apply(context.Background())
	if err := m.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("Down 未调用")
	}
}

func TestRollback_NoDown(t *testing.T) {
	store := NewMemStore()
	list := []Migration{{Name: "a", Up: func(_ context.Context, _ *Migration) error { return nil }}}
	m := New(store, list)
	m.Apply(context.Background())
	if err := m.Rollback(context.Background()); err == nil {
		t.Fatal("应报错")
	}
}

func TestMemStore_PopEmpty(t *testing.T) {
	s := NewMemStore()
	if _, err := s.Pop(context.Background()); !errors.Is(err, ErrEmpty) {
		t.Fatal("应 ErrEmpty")
	}
}

func TestMemStore_AppendList(t *testing.T) {
	s := NewMemStore()
	s.Append(context.Background(), Record{Index: 0})
	list, _ := s.List(context.Background())
	if len(list) != 1 {
		t.Fatal("append")
	}
}
