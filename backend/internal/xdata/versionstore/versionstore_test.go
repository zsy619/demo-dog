package versionstore

import "testing"

func TestSetGet(t *testing.T) {
	s := New(5)
	s.Set("a", 1)
	v, _ := s.Get("a")
	if v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestVersions(t *testing.T) {
	s := New(5)
	s.Set("a", 1)
	s.Set("a", 2)
	s.Set("a", 3)
	v := s.Versions("a")
	if len(v) != 3 || v[0] != 1 {
		t.Fatal("vers", v)
	}
}

func TestGetAt(t *testing.T) {
	s := New(5)
	s.Set("a", 1)
	s.Set("a", 2)
	s.Set("a", 3)
	v, ok := s.GetAt("a", 2)
	if !ok || v.(int) != 2 {
		t.Fatal("at", v)
	}
	if _, ok := s.GetAt("a", 99); ok {
		t.Fatal("missing")
	}
}

func TestCap(t *testing.T) {
	s := New(2)
	s.Set("a", 1)
	s.Set("a", 2)
	s.Set("a", 3)
	if len(s.Versions("a")) != 2 {
		t.Fatal("cap")
	}
}

func TestDelete(t *testing.T) {
	s := New(2)
	s.Set("a", 1)
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("del")
	}
}
