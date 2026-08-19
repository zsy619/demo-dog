package loadbal

import (
	"errors"
	"testing"
)

func TestRoundRobin(t *testing.T) {
	b := New(RoundRobin, []Host{{Addr: "a"}, {Addr: "b"}, {Addr: "c"}})
	expected := []string{"a", "b", "c", "a", "b"}
	for _, e := range expected {
		got, err := b.Next()
		if err != nil {
			t.Fatal(err)
		}
		if got != e {
			t.Fatal("got", got, "want", e)
		}
	}
}

func TestRandom(t *testing.T) {
	b := New(Random, []Host{{Addr: "a"}, {Addr: "b"}})
	seen := map[string]int{}
	for i := 0; i < 100; i++ {
		a, _ := b.Next()
		seen[a]++
	}
	if len(seen) != 2 {
		t.Fatal("random")
	}
}

func TestWeighted(t *testing.T) {
	b := New(WeightedRoundRobin, []Host{{Addr: "a", Weight: 1}, {Addr: "b", Weight: 3}})
	counts := map[string]int{}
	for i := 0; i < 40; i++ {
		a, _ := b.Next()
		counts[a]++
	}
	if counts["a"] == 0 || counts["b"] == 0 {
		t.Fatal("weighted")
	}
	if counts["b"] < counts["a"] {
		t.Fatal("权重错")
	}
}

func TestNoHosts(t *testing.T) {
	b := New(RoundRobin, nil)
	if _, err := b.Next(); !errors.Is(err, ErrNoHosts) {
		t.Fatal("应 ErrNoHosts")
	}
}

func TestUpdate(t *testing.T) {
	b := New(RoundRobin, []Host{{Addr: "a"}})
	b.Update([]Host{{Addr: "x"}, {Addr: "y"}})
	if b.Count() != 2 {
		t.Fatal("update")
	}
}

func TestHosts(t *testing.T) {
	b := New(RoundRobin, []Host{{Addr: "a"}, {Addr: "b"}})
	h := b.Hosts()
	if len(h) != 2 || h[0].Addr != "a" {
		t.Fatal("hosts")
	}
}

func TestWeighted_Zero(t *testing.T) {
	b := New(WeightedRoundRobin, []Host{{Addr: "a"}, {Addr: "b"}})
	a, err := b.Next()
	if err != nil || a == "" {
		t.Fatal("zero weight")
	}
}
