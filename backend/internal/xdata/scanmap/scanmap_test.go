package scanmap

import "testing"

func TestNext(t *testing.T) {
	s := New(map[string]int{"b": 2, "a": 1, "c": 3})
	k, v, ok := s.Next()
	if !ok || k != "a" || v != 1 {
		t.Fatal("first", k, v)
	}
}

func TestAll(t *testing.T) {
	s := New(map[string]int{"b": 2, "a": 1, "c": 3})
	keys := []string{}
	for s.Len() > 0 {
		k, _, ok := s.Next()
		if !ok {
			break
		}
		keys = append(keys, k)
	}
	if len(keys) != 3 || keys[0] != "a" {
		t.Fatal("all", keys)
	}
}

func TestReset(t *testing.T) {
	s := New(map[string]int{"a": 1})
	s.Next()
	s.Reset()
	if s.Pos() != 0 {
		t.Fatal("reset")
	}
}

func TestSeek(t *testing.T) {
	s := New(map[string]int{"a": 1, "b": 2, "c": 3})
	s.Seek("b")
	k, _, _ := s.Next()
	if k != "b" {
		t.Fatal("seek", k)
	}
}

func TestSlice(t *testing.T) {
	s := New(map[string]int{"b": 2, "a": 1})
	sl := s.Slice()
	if sl[0] != "a" || sl[1] != "b" {
		t.Fatal("slice", sl)
	}
}
