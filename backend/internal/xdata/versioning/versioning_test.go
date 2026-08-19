package versioning

import (
	"errors"
	"testing"
)

func TestVersion_Bump(t *testing.T) {
	v := New()
	if v.Bump() != 2 {
		t.Fatal("bump")
	}
}

func TestVersion_CAS(t *testing.T) {
	v := New()
	cur := v.Current()
	if !v.CompareAndSwap(cur, cur+1) {
		t.Fatal("cas")
	}
	if v.Current() != cur+1 {
		t.Fatal("cur")
	}
}

func TestVersion_CASFail(t *testing.T) {
	v := New()
	if v.CompareAndSwap(99, 100) {
		t.Fatal("应 false")
	}
}

func TestGuard_Update(t *testing.T) {
	g := NewGuard(0)
	_, ver := g.Get()
	if err := g.Update(ver, func(v int) int { return v + 1 }); err != nil {
		t.Fatal(err)
	}
	v, _ := g.Get()
	if v != 1 {
		t.Fatal("v")
	}
}

func TestGuard_Stale(t *testing.T) {
	g := NewGuard(0)
	g.Force(func(v int) int { return v })
	err := g.Update(1, func(v int) int { return v + 1 })
	if !errors.Is(err, ErrStale) {
		t.Fatal("应 stale")
	}
}

func TestGuard_Force(t *testing.T) {
	g := NewGuard(0)
	g.Force(func(v int) int { return 5 })
	v, _ := g.Get()
	if v != 5 {
		t.Fatal("force")
	}
}
