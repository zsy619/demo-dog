package segment

import "testing"

func TestPutGet(t *testing.T) {
	c := New()
	c.Put("users", "u1", 1)
	if v, ok := c.Get("users", "u1"); !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestMatch(t *testing.T) {
	c := New()
	c.Put("users", "alice:email", "a@b.com")
	v, ok := c.Match("users", "alice", "email")
	if !ok || v.(string) != "a@b.com" {
		t.Fatal("match", v)
	}
}

func TestDelete(t *testing.T) {
	c := New()
	c.Put("a", "k", 1)
	c.Delete("a", "k")
	if _, ok := c.Get("a", "k"); ok {
		t.Fatal("del")
	}
}

func TestClearPrefix(t *testing.T) {
	c := New()
	c.Put("a", "x", 1)
	c.ClearPrefix("a")
	if _, ok := c.Get("a", "x"); ok {
		t.Fatal("clear")
	}
}

func TestKeys(t *testing.T) {
	c := New()
	c.Put("a", "k1", 1)
	c.Put("a", "k2", 2)
	k := c.Keys("a")
	if len(k) != 2 {
		t.Fatal("keys", k)
	}
}
