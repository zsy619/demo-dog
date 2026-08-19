package radix

import (
	"testing"
)

func TestInsertLookup(t *testing.T) {
	t1 := New()
	t1.Insert("foo", 1)
	if v := t1.Lookup("foo"); v != 1 {
		t.Fatal("lookup")
	}
}

func TestLookup_Missing(t *testing.T) {
	t1 := New()
	if v := t1.Lookup("missing"); v != nil {
		t.Fatal("missing")
	}
}

func TestInsert_Empty(t *testing.T) {
	t1 := New()
	t1.Insert("", 1)
	if t1.Len() != 0 {
		t.Fatal("empty")
	}
}

func TestPrefixCompression(t *testing.T) {
	t1 := New()
	t1.Insert("foo", 1)
	t1.Insert("foobar", 2)
	if t1.Len() != 2 {
		t.Fatal("len")
	}
	if v := t1.Lookup("foo"); v != 1 {
		t.Fatal("foo")
	}
	if v := t1.Lookup("foobar"); v != 2 {
		t.Fatal("foobar")
	}
}

func TestInsert_Rewrite(t *testing.T) {
	t1 := New()
	t1.Insert("foo", 1)
	t1.Insert("foo", 2)
	if v := t1.Lookup("foo"); v != 2 {
		t.Fatal("rewrite")
	}
}

func TestInsert_MultipleKeys(t *testing.T) {
	t1 := New()
	for _, k := range []string{"a", "ab", "abc", "abcd", "b", "bc"} {
		t1.Insert(k, k)
	}
	for _, k := range []string{"a", "ab", "abc", "abcd", "b", "bc"} {
		if v := t1.Lookup(k); v != k {
			t.Fatal(k)
		}
	}
}

func TestMatchPattern(t *testing.T) {
	t1 := New()
	t1.Insert("api/v1", 1)
	t1.Insert("api/v2", 2)
	v, ok := t1.MatchPattern("api/v1*")
	if !ok || v != 1 {
		t.Fatal("match")
	}
	v, ok = t1.MatchPattern("api/v2*")
	if !ok || v != 2 {
		t.Fatal("match 2")
	}
	if _, ok := t1.MatchPattern("missing*"); ok {
		t.Fatal("missing")
	}
}

func TestMatchPattern_NoStar(t *testing.T) {
	t1 := New()
	t1.Insert("a", 1)
	v, ok := t1.MatchPattern("a")
	if !ok || v != 1 {
		t.Fatal("no star")
	}
}

func TestLen(t *testing.T) {
	t1 := New()
	if t1.Len() != 0 {
		t.Fatal("empty")
	}
	t1.Insert("a", 1)
	t1.Insert("b", 2)
	if t1.Len() != 2 {
		t.Fatal("len")
	}
}

func TestIPStyle(t *testing.T) {
	t1 := New()
	t1.Insert("10.0.0.0/8", "corp")
	t1.Insert("192.168.0.0/16", "private")
	if v := t1.Lookup("10.0.0.0/8"); v != "corp" {
		t.Fatal("corp")
	}
	if v := t1.Lookup("192.168.0.0/16"); v != "private" {
		t.Fatal("private")
	}
}
