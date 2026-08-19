package sempool

import (
	"errors"
	"testing"
)

func TestAcquire(t *testing.T) {
	p := New([]Slot{{Name: "db", Weight: 10}, {Name: "cache", Weight: 5}})
	name, release, err := p.Acquire(3)
	if err != nil {
		t.Fatal(err)
	}
	if name != "db" {
		t.Fatal("应选第一个能容纳的")
	}
	release()
	if p.Available()[0].Weight != 10 {
		t.Fatal("释放失败")
	}
}

func TestAcquire_Full(t *testing.T) {
	p := New([]Slot{{Name: "db", Weight: 1}})
	p.Acquire(1)
	if _, _, err := p.Acquire(1); !errors.Is(err, ErrFull) {
		t.Fatal("应 ErrFull")
	}
}

func TestAcquire_Zero(t *testing.T) {
	p := New([]Slot{{Name: "a", Weight: 1}})
	name, _, err := p.Acquire(0)
	if err != nil || name != "" {
		t.Fatal("n<=0 不应消耗")
	}
}

func TestAddSlot(t *testing.T) {
	p := New([]Slot{{Name: "a", Weight: 1}})
	p.AddSlot(Slot{Name: "b", Weight: 5})
	if p.Total() != 6 {
		t.Fatal("add")
	}
}

func TestAvailable(t *testing.T) {
	p := New([]Slot{{Name: "a", Weight: 5}})
	p.Acquire(2)
	av := p.Available()
	if av[0].Weight != 3 {
		t.Fatal("avail")
	}
}

func TestTotal(t *testing.T) {
	p := New([]Slot{{Name: "a", Weight: 3}, {Name: "b", Weight: 7}})
	if p.Total() != 10 {
		t.Fatal("total")
	}
}

func TestUsed(t *testing.T) {
	p := New([]Slot{{Name: "a", Weight: 10}})
	p.Acquire(4)
	if p.Used() != 4 {
		t.Fatal("used")
	}
}
