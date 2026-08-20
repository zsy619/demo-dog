package lsm

import (
	"testing"
)

func TestMemtable_PutGet(t *testing.T) {
	m := NewMemtable()
	m.Put("a", []byte("1"))
	v, ok, tomb := m.Get("a")
	if !ok || tomb || string(v) != "1" {
		t.Fatal("get")
	}
}

func TestMemtable_Missing(t *testing.T) {
	m := NewMemtable()
	if _, ok, _ := m.Get("missing"); ok {
		t.Fatal("missing")
	}
}

func TestMemtable_Delete(t *testing.T) {
	m := NewMemtable()
	m.Put("a", []byte("1"))
	m.Delete("a")
	if _, ok, tomb := m.Get("a"); ok || !tomb {
		t.Fatal("should be tombstone")
	}
}

func TestMemtable_Replace(t *testing.T) {
	m := NewMemtable()
	m.Put("a", []byte("1"))
	m.Put("a", []byte("2"))
	v, ok, _ := m.Get("a")
	if !ok || string(v) != "2" {
		t.Fatal("replace")
	}
	if m.Len() != 1 {
		t.Fatal("len")
	}
}

func TestMemtable_Ordering(t *testing.T) {
	m := NewMemtable()
	m.Put("c", []byte("3"))
	m.Put("a", []byte("1"))
	m.Put("b", []byte("2"))
	if m.entries[0].Key != "a" || m.entries[1].Key != "b" || m.entries[2].Key != "c" {
		t.Fatal("not sorted")
	}
}

func TestSortedRun_Get(t *testing.T) {
	r := NewSortedRun([]Entry{
		{Key: "a", Value: []byte("1")},
		{Key: "b", Value: []byte("2"), Tombstone: true},
	})
	if v, ok, _ := r.Get("a"); !ok || string(v) != "1" {
		t.Fatal("a")
	}
	if _, ok, tomb := r.Get("b"); ok || !tomb {
		t.Fatal("b tomb")
	}
}

func TestStringTable_PutGet(t *testing.T) {
	s := NewStringTable()
	s.Put("a", []byte("1"))
	v, ok := s.Get("a")
	if !ok || string(v) != "1" {
		t.Fatal("get")
	}
}

func TestStringTable_Flush(t *testing.T) {
	s := NewStringTable()
	s.Put("a", []byte("1"))
	s.Put("b", []byte("2"))
	if s.Flush() != 1 {
		t.Fatal("flush")
	}
	if s.MemLen() != 0 {
		t.Fatal("mem should be empty")
	}
	if s.RunCount() != 1 {
		t.Fatal("runs")
	}
	v, ok := s.Get("a")
	if !ok || string(v) != "1" {
		t.Fatal("after flush")
	}
}

func TestStringTable_NewerMem(t *testing.T) {
	s := NewStringTable()
	s.Put("a", []byte("1"))
	s.Flush()
	s.Put("a", []byte("2"))
	v, ok := s.Get("a")
	if !ok || string(v) != "2" {
		t.Fatal("mem should win")
	}
}

func TestStringTable_TombstoneMem(t *testing.T) {
	s := NewStringTable()
	s.Put("a", []byte("1"))
	s.Flush()
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("mem tombstone should hide run value")
	}
}

func TestStringTable_Missing(t *testing.T) {
	s := NewStringTable()
	if _, ok := s.Get("missing"); ok {
		t.Fatal("missing")
	}
}

func TestStringTable_MultiRuns(t *testing.T) {
	s := NewStringTable()
	s.Put("a", []byte("1"))
	s.Flush()
	s.Put("b", []byte("2"))
	s.Flush()
	s.Put("c", []byte("3"))
	if s.RunCount() != 2 {
		t.Fatal("runs")
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, ok := s.Get(k); !ok {
			t.Fatalf("missing %s", k)
		}
	}
}

func TestCopyBytes(t *testing.T) {
	b := []byte("x")
	c := copyBytes(b)
	c[0] = 'y'
	if b[0] != 'x' {
		t.Fatal("should be independent")
	}
	if copyBytes(nil) != nil {
		t.Fatal("nil")
	}
}
