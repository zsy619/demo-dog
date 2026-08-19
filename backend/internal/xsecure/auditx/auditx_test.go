package auditx

import (
	"testing"
)

func TestRecord(t *testing.T) {
	l := New()
	e := l.Record("alice", "login", "self", "success", nil)
	if e.Seq != 1 {
		t.Fatal("seq")
	}
}

func TestHistory(t *testing.T) {
	l := New()
	l.Record("alice", "x", "y", "success", nil)
	l.Record("bob", "y", "z", "fail", nil)
	if len(l.History()) != 2 {
		t.Fatal("history")
	}
}

func TestTail(t *testing.T) {
	l := New()
	for i := 0; i < 5; i++ {
		l.Record("a", "x", "y", "success", nil)
	}
	if len(l.Tail(2)) != 2 {
		t.Fatal("tail")
	}
}

func TestFilter(t *testing.T) {
	l := New()
	l.Record("alice", "x", "y", "success", nil)
	l.Record("bob", "x", "y", "success", nil)
	if len(l.Filter("alice")) != 1 {
		t.Fatal("filter")
	}
}

func TestStats(t *testing.T) {
	l := New()
	l.Record("a", "x", "y", "success", nil)
	l.Record("b", "x", "y", "fail", nil)
	s := l.Stats()
	if s.Total != 2 || s.Failures != 1 {
		t.Fatal("stats")
	}
}

func TestClear(t *testing.T) {
	l := New()
	l.Record("a", "x", "y", "success", nil)
	l.Clear()
	if len(l.History()) != 0 {
		t.Fatal("clear")
	}
}

func TestMeta(t *testing.T) {
	l := New()
	e := l.Record("a", "x", "y", "success", map[string]any{"k": 1})
	if e.Meta["k"].(int) != 1 {
		t.Fatal("meta")
	}
}
