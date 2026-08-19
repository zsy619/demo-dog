package feature_flag

import "testing"

func TestEnableDisable(t *testing.T) {
	s := New()
	s.Enable("x")
	if !s.IsEnabled("x") {
		t.Fatal("enable")
	}
	s.Disable("x")
	if s.IsEnabled("x") {
		t.Fatal("disable")
	}
}

func TestSet(t *testing.T) {
	s := New()
	s.Set("a", Flag{On: true, Rollout: 50})
	f, ok := s.Get("a")
	if !ok || !f.On || f.Rollout != 50 {
		t.Fatal("set", f)
	}
}

func TestDelete(t *testing.T) {
	s := New()
	s.Set("a", Flag{On: true})
	s.Delete("a")
	if s.IsEnabled("a") {
		t.Fatal("del")
	}
}

func TestAll(t *testing.T) {
	s := New()
	s.Set("a", Flag{On: true})
	s.Set("b", Flag{On: false})
	all := s.All()
	if len(all) != 2 {
		t.Fatal("all", len(all))
	}
}

func TestMiss(t *testing.T) {
	s := New()
	if s.IsEnabled("missing") {
		t.Fatal("miss")
	}
}
