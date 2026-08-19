package docstore

import (
	"testing"
)

func TestPutGet(t *testing.T) {
	s := New()
	s.Put(&Doc{ID: "a", Values: map[string]any{"x": "y"}})
	d, ok := s.Get("a")
	if !ok || d.Values["x"] != "y" {
		t.Fatal("get")
	}
}

func TestPut_NoID(t *testing.T) {
	s := New()
	if err := s.Put(&Doc{Values: map[string]any{"x": "y"}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPut_Replace(t *testing.T) {
	s := New()
	s.Put(&Doc{ID: "a", Values: map[string]any{"status": "active"}})
	s.Put(&Doc{ID: "a", Values: map[string]any{"status": "banned"}})
	d, _ := s.Get("a")
	if d.Values["status"] != "banned" {
		t.Fatal("replace")
	}
	// Index should not retain the old value.
	if docs := s.FindByIndex("status", "active"); len(docs) != 0 {
		t.Fatal("stale index")
	}
}

func TestGet_Missing(t *testing.T) {
	s := New()
	if _, ok := s.Get("missing"); ok {
		t.Fatal("expected miss")
	}
}

func TestDelete(t *testing.T) {
	s := New()
	s.Put(&Doc{ID: "a", Values: map[string]any{"k": "v"}})
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("a"); ok {
		t.Fatal("should be deleted")
	}
	if docs := s.FindByIndex("k", "v"); len(docs) != 0 {
		t.Fatal("index should be cleared")
	}
}

func TestDelete_Missing(t *testing.T) {
	s := New()
	if err := s.Delete("missing"); err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}
}

func TestFindByIndex(t *testing.T) {
	s := New()
	s.Put(&Doc{ID: "a", Values: map[string]any{"tenant": "acme"}})
	s.Put(&Doc{ID: "b", Values: map[string]any{"tenant": "acme"}})
	s.Put(&Doc{ID: "c", Values: map[string]any{"tenant": "globex"}})
	acme := s.FindByIndex("tenant", "acme")
	if len(acme) != 2 {
		t.Fatalf("acme: %d", len(acme))
	}
	globex := s.FindByIndex("tenant", "globex")
	if len(globex) != 1 {
		t.Fatalf("globex: %d", len(globex))
	}
	miss := s.FindByIndex("tenant", "missing")
	if len(miss) != 0 {
		t.Fatal("missing")
	}
	noIndex := s.FindByIndex("unknown", "x")
	if len(noIndex) != 0 {
		t.Fatal("no index")
	}
}

func TestFindByIndex_NonStringIgnored(t *testing.T) {
	s := New()
	s.Put(&Doc{ID: "a", Values: map[string]any{"n": 42}})
	if docs := s.FindByIndex("n", "42"); len(docs) != 0 {
		t.Fatal("non-string should not be indexed")
	}
}

func TestCount(t *testing.T) {
	s := New()
	if s.Count() != 0 {
		t.Fatal("empty")
	}
	s.Put(&Doc{ID: "a", Values: map[string]any{}})
	s.Put(&Doc{ID: "b", Values: map[string]any{}})
	if s.Count() != 2 {
		t.Fatal("count")
	}
}

func TestListIDs(t *testing.T) {
	s := New()
	s.Put(&Doc{ID: "b", Values: map[string]any{}})
	s.Put(&Doc{ID: "a", Values: map[string]any{}})
	ids := s.ListIDs()
	if len(ids) != 2 || ids[0] != "a" {
		t.Fatalf("ids: %v", ids)
	}
}
