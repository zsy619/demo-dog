package recency

import "testing"

func TestTouch(t *testing.T) {
	t0 := New()
	t0.Touch("a", 100)
	v, ok := t0.Get("a")
	if !ok || v != 100 {
		t.Fatal("touch")
	}
}

func TestOldest(t *testing.T) {
	t0 := New()
	t0.Touch("a", 100)
	t0.Touch("b", 50)
	t0.Touch("c", 200)
	k, ts, ok := t0.Oldest()
	if !ok || k != "b" || ts != 50 {
		t.Fatal("oldest", k, ts)
	}
}

func TestPurge(t *testing.T) {
	t0 := New()
	t0.Touch("a", 50)
	t0.Touch("b", 100)
	t0.Touch("c", 150)
	n := t0.PurgeOlderThan(100)
	if n != 1 {
		t.Fatal("purge", n)
	}
	if t0.Len() != 2 {
		t.Fatal("len")
	}
}

func TestEmpty(t *testing.T) {
	t0 := New()
	if _, _, ok := t0.Oldest(); ok {
		t.Fatal("oldest empty")
	}
}

func TestDelete(t *testing.T) {
	t0 := New()
	t0.Touch("a", 100)
	t0.Delete("a")
	if _, ok := t0.Get("a"); ok {
		t.Fatal("del")
	}
}
