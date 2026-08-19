package sortedkv

import "testing"

func TestSortedKeys(t *testing.T) {
	k := New()
	k.Set("c", 3)
	k.Set("a", 1)
	k.Set("b", 2)
	keys := k.SortedKeys()
	if len(keys) != 3 || keys[0] != "a" {
		t.Fatal("sort", keys)
	}
}

func TestGet(t *testing.T) {
	k := New()
	k.Set("a", 1)
	v, ok := k.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestRange(t *testing.T) {
	k := New()
	k.Set("a", 1)
	k.Set("b", 2)
	sum := 0
	k.Range(func(_ string, v any) bool { sum += v.(int); return true })
	if sum != 3 {
		t.Fatal("range", sum)
	}
}

func TestDelete(t *testing.T) {
	k := New()
	k.Set("a", 1)
	k.Delete("a")
	if _, ok := k.Get("a"); ok {
		t.Fatal("del")
	}
}
