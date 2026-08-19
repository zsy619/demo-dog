package routex

import "testing"

func TestMatch(t *testing.T) {
	tbl := New()
	tbl.Add("/api/v1", "v1")
	tbl.Add("/api/v2", "v2")
	tbl.Add("/", "root")
	v, ok := tbl.Match("/api/v1/users")
	if !ok || v != "v1" {
		t.Fatal("match", v)
	}
}

func TestLongest(t *testing.T) {
	tbl := New()
	tbl.Add("/api", "api")
	tbl.Add("/api/v1", "v1")
	v, _ := tbl.Match("/api/v1/users")
	if v != "v1" {
		t.Fatal("longest", v)
	}
}

func TestMiss(t *testing.T) {
	tbl := New()
	if _, ok := tbl.Match("/x"); ok {
		t.Fatal("miss")
	}
}

func TestClear(t *testing.T) {
	tbl := New()
	tbl.Add("/a", "x")
	tbl.Clear()
	if tbl.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestRoot(t *testing.T) {
	tbl := New()
	tbl.Add("/", "root")
	v, ok := tbl.Match("/anything")
	if !ok || v != "root" {
		t.Fatal("root", v)
	}
}
