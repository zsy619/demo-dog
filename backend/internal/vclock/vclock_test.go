package vclock

import (
	"testing"
)

func TestTick(t *testing.T) {
	c := New()
	if c.Tick("a") != 1 {
		t.Fatal("tick1")
	}
	if c.Tick("a") != 2 {
		t.Fatal("tick2")
	}
	if c.Get("a") != 2 {
		t.Fatal("get")
	}
}

func TestSet(t *testing.T) {
	c := New()
	c.Set("a", 5)
	if c.Get("a") != 5 {
		t.Fatal("set")
	}
}

func TestUpdate_Max(t *testing.T) {
	a := New()
	b := New()
	a.Set("x", 1)
	a.Set("y", 3)
	b.Set("y", 2)
	b.Set("z", 4)
	a.Update(b)
	if a.Get("x") != 1 || a.Get("y") != 3 || a.Get("z") != 4 {
		t.Fatal("update")
	}
}

func TestCompare_Less(t *testing.T) {
	a := New()
	b := New()
	a.Set("x", 1)
	b.Set("x", 2)
	if a.Compare(b) != -1 {
		t.Fatal("less")
	}
	if b.Compare(a) != 1 {
		t.Fatal("greater")
	}
}

func TestCompare_Equal(t *testing.T) {
	a := New()
	b := New()
	a.Set("x", 3)
	b.Set("x", 3)
	if a.Compare(b) != 0 {
		t.Fatal("equal")
	}
}

func TestCompare_Concurrent(t *testing.T) {
	a := New()
	b := New()
	a.Set("x", 2)
	b.Set("y", 2)
	if a.Compare(b) != 2 {
		t.Fatal("concurrent")
	}
}

func TestSnapshot(t *testing.T) {
	c := New()
	c.Set("a", 1)
	c.Set("b", 2)
	s := c.Snapshot()
	if s["a"] != 1 || s["b"] != 2 {
		t.Fatal("snapshot")
	}
	// Mutating snapshot doesn't affect clock.
	delete(s, "a")
	if c.Get("a") != 1 {
		t.Fatal("should be independent")
	}
}

func TestFromSnapshot(t *testing.T) {
	c := New()
	c.FromSnapshot(map[string]uint64{"a": 5})
	if c.Get("a") != 5 {
		t.Fatal("from snapshot")
	}
}

func TestNodes(t *testing.T) {
	c := New()
	c.Set("b", 1)
	c.Set("a", 1)
	c.Set("c", 1)
	nodes := c.Nodes()
	if len(nodes) != 3 || nodes[0] != "a" {
		t.Fatalf("nodes: %v", nodes)
	}
}
