package partition

import "testing"

func TestEmpty(t *testing.T) {
	r := New()
	if r.Get("x") != "" {
		t.Fatal("empty")
	}
}

func TestAddGet(t *testing.T) {
	r := New()
	r.Add("a")
	r.Add("b")
	if r.Get("k") == "" {
		t.Fatal("get")
	}
}

func TestRemove(t *testing.T) {
	r := New()
	r.Add("a")
	r.Add("b")
	r.Remove("a")
	if r.Len() != 1 {
		t.Fatal("remove")
	}
}

func TestLen(t *testing.T) {
	r := New()
	if r.Len() != 0 {
		t.Fatal("empty")
	}
	r.Add("a")
	if r.Len() != 1 {
		t.Fatal("len")
	}
}

func TestNodes(t *testing.T) {
	r := New()
	r.Add("a")
	r.Add("b")
	r.Add("c")
	n := r.Nodes()
	if len(n) != 3 {
		t.Fatal("nodes", n)
	}
}

func TestDistribution(t *testing.T) {
	r := New()
	r.Add("node-a")
	r.Add("node-b")
	hits := map[string]int{}
	for i := 0; i < 1000; i++ {
		hits[r.Get(string(rune(100+i)))]++
	}
	if len(hits) != 2 {
		t.Fatal("dist", hits)
	}
}
