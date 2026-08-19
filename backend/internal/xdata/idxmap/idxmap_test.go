package idxmap

import "testing"

func TestSearch(t *testing.T) {
	i := New()
	i.Add("d1", "a", "b")
	i.Add("d2", "b", "c")
	i.Add("d3", "c", "d")
	r := i.Search("b")
	if len(r) != 2 {
		t.Fatal("search", r)
	}
}

func TestSearchAll(t *testing.T) {
	i := New()
	i.Add("d1", "a", "b", "c")
	i.Add("d2", "a", "b")
	i.Add("d3", "a")
	r := i.SearchAll("a", "b")
	if len(r) != 2 {
		t.Fatal("all", r)
	}
}

func TestMiss(t *testing.T) {
	i := New()
	if r := i.Search("missing"); len(r) != 0 {
		t.Fatal("miss")
	}
}

func TestCount(t *testing.T) {
	i := New()
	i.Add("d1", "a")
	i.Add("d2", "a")
	if i.DocCount() != 2 {
		t.Fatal("count")
	}
}
