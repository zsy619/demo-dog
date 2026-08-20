package ring

import (
	"fmt"
	"testing"
)

func TestLookup_Empty(t *testing.T) {
	r := New(10)
	if _, err := r.Lookup("x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLookup_Stable(t *testing.T) {
	r := New(100)
	r.Add("a")
	r.Add("b")
	r.Add("c")
	for _, k := range []string{"x", "y", "z", "k1", "k2"} {
		n1, _ := r.Lookup(k)
		n2, _ := r.Lookup(k)
		if n1 != n2 {
			t.Fatalf("unstable: %s -> %s vs %s", k, n1, n2)
		}
	}
}

func TestLookup_AlwaysHitsNode(t *testing.T) {
	r := New(50)
	r.Add("a")
	r.Add("b")
	r.Add("c")
	for _, k := range []string{"x", "y", "z"} {
		n, err := r.Lookup(k)
		if err != nil {
			t.Fatal(err)
		}
		if n != "a" && n != "b" && n != "c" {
			t.Fatalf("unknown node: %s", n)
		}
	}
}

func TestRemove(t *testing.T) {
	r := New(50)
	r.Add("a")
	r.Add("b")
	r.Remove("a")
	if r.Size() != 1 {
		t.Fatal("size after remove")
	}
	n, err := r.Lookup("x")
	if err != nil {
		t.Fatal(err)
	}
	if n != "b" {
		t.Fatalf("expected b, got %s", n)
	}
}

func TestLookupN(t *testing.T) {
	r := New(50)
	r.Add("a")
	r.Add("b")
	r.Add("c")
	n, err := r.LookupN("k", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(n) != 2 {
		t.Fatalf("expected 2, got %d", len(n))
	}
	if n[0] == n[1] {
		t.Fatal("expected distinct")
	}
}

func TestNodes(t *testing.T) {
	r := New(10)
	r.Add("a")
	r.Add("b")
	r.Add("a")
	nodes := r.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("nodes: %v", nodes)
	}
}

func TestDistribution_Balanced(t *testing.T) {
	r := New(100)
	r.Add("a")
	r.Add("b")
	r.Add("c")
	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%d", i)
	}
	dist := r.Distribution(keys)
	if len(dist) != 3 {
		t.Fatalf("dist: %v", dist)
	}
	for n, frac := range dist {
		if frac < 0.15 || frac > 0.55 {
			t.Fatalf("unbalanced for %s: %v", n, frac)
		}
	}
}

func TestDistribution_Empty(t *testing.T) {
	r := New(10)
	r.Add("a")
	if d := r.Distribution(nil); d != nil {
		t.Fatal("empty keys")
	}
}

func TestHashKey_Stable(t *testing.T) {
	if hashKey("abc") != hashKey("abc") {
		t.Fatal("hash should be stable")
	}
}
