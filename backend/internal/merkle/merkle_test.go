package merkle

import (
	"testing"
)

func TestRoot_Empty(t *testing.T) {
	t1 := New(nil)
	if t1.Root() == "" {
		t.Fatal("empty root")
	}
}

func TestRoot_Single(t *testing.T) {
	t1 := New([]string{"a"})
	if t1.Root() == "" {
		t.Fatal("root")
	}
}

func TestRoot_Deterministic(t *testing.T) {
	t1 := New([]string{"a", "b", "c"})
	t2 := New([]string{"c", "b", "a"})
	if t1.Root() != t2.Root() {
		t.Fatal("order should not matter")
	}
}

func TestRoot_Different(t *testing.T) {
	t1 := New([]string{"a", "b"})
	t2 := New([]string{"a", "c"})
	if t1.Root() == t2.Root() {
		t.Fatal("should differ")
	}
}

func TestEqual(t *testing.T) {
	t1 := New([]string{"a", "b"})
	t2 := New([]string{"b", "a"})
	if !t1.Equal(t2) {
		t.Fatal("equal")
	}
}

func TestDiff(t *testing.T) {
	t1 := New([]string{"a", "b"})
	t2 := New([]string{"a", "b", "c"})
	diff := t1.Diff(t2)
	if len(diff) != 1 || diff[0] != "c" {
		t.Fatal("diff")
	}
}

func TestKeys(t *testing.T) {
	t1 := New([]string{"c", "a", "b"})
	keys := t1.Keys()
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Fatal("keys")
	}
}

func TestProof(t *testing.T) {
	t1 := New([]string{"a", "b", "c", "d"})
	p := t1.Proof("a")
	if p == nil {
		t.Fatal("proof")
	}
	if !t1.VerifyProof(p) {
		t.Fatal("verify")
	}
}

func TestProof_Missing(t *testing.T) {
	t1 := New([]string{"a", "b"})
	if p := t1.Proof("missing"); p != nil {
		t.Fatal("missing")
	}
}

func TestLeafHash(t *testing.T) {
	h1 := leafHash("a")
	h2 := leafHash("a")
	if hexEq(h1, h2) == false {
		t.Fatal("leaf stable")
	}
}

func hexEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
