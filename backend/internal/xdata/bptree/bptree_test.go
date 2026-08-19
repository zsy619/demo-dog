package bptree

import "testing"

func TestPutGet(t *testing.T) {
	t0 := New(8)
	t0.Put(1, "a")
	v, ok := t0.Get(1)
	if !ok || v.(string) != "a" {
		t.Fatal("get")
	}
}

func TestUpdate(t *testing.T) {
	t0 := New(8)
	t0.Put(1, "a")
	t0.Put(1, "b")
	v, _ := t0.Get(1)
	if v.(string) != "b" {
		t.Fatal("upd")
	}
}

func TestRange(t *testing.T) {
	t0 := New(8)
	for i := int64(0); i < 10; i++ {
		t0.Put(i, i)
	}
	r := t0.Range(3, 6)
	if len(r) != 4 {
		t.Fatal("range", len(r))
	}
}

func TestMiss(t *testing.T) {
	t0 := New(8)
	if _, ok := t0.Get(99); ok {
		t.Fatal("miss")
	}
}

func TestKeys(t *testing.T) {
	t0 := New(8)
	t0.Put(2, "")
	t0.Put(1, "")
	t0.Put(3, "")
	ks := t0.Keys()
	if ks[0] != 1 || ks[2] != 3 {
		t.Fatal("keys", ks)
	}
}
