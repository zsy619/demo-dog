package skiptable

import "testing"

func TestSetGet(t *testing.T) {
	s := New()
	s.Set("a", 1)
	v, ok := s.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get", v)
	}
}

func TestUpdate(t *testing.T) {
	s := New()
	s.Set("a", 1)
	s.Set("a", 2)
	v, _ := s.Get("a")
	if v.(int) != 2 {
		t.Fatal("update")
	}
}

func TestDelete(t *testing.T) {
	s := New()
	s.Set("a", 1)
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestLen(t *testing.T) {
	s := New()
	for i := 0; i < 100; i++ {
		s.Set(string(rune('a'+i%26)), i)
	}
	if s.Len() != 26 {
		t.Fatal("len", s.Len())
	}
}

func TestRange(t *testing.T) {
	s := New()
	s.Set("b", 2)
	s.Set("a", 1)
	s.Set("c", 3)
	order := []string{}
	s.Range(func(k string, _ any) bool { order = append(order, k); return true })
	if len(order) != 3 || order[0] != "a" {
		t.Fatal("order", order)
	}
}
