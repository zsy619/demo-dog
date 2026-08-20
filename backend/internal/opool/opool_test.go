package opool

import (
	"testing"
)

type box struct{ v int }

func TestGetPut(t *testing.T) {
	p := New(func() *box { return &box{} }, func(b *box) { b.v = 0 })
	b := p.Get()
	b.v = 1
	p.Put(b)
	b2 := p.Get()
	if b2.v != 0 {
		t.Fatal("reset")
	}
}

func TestCreated(t *testing.T) {
	p := New(func() *box { return &box{} }, nil)
	b := p.Get()
	p.Put(b)
	st := p.Stats()
	if st.Created == 0 {
		t.Fatal("created")
	}
}

func TestReused(t *testing.T) {
	p := New(func() *box { return &box{} }, nil)
	b := p.Get()
	p.Put(b)
	b2 := p.Get()
	p.Put(b2)
	st := p.Stats()
	if st.Reused == 0 {
		t.Fatal("reused")
	}
}

func TestDiscard(t *testing.T) {
	p := New(func() *box { return &box{} }, nil)
	p.Discard()
	if p.Stats().Discarded != 1 {
		t.Fatal("discard")
	}
}

func TestNilReset(t *testing.T) {
	// 验证 nil reset 不会清零对象字段。
	p := New(func() *box { return &box{} }, nil)
	b := p.Get()
	b.v = 5
	p.Put(b)
	st := p.Stats()
	if st.Created == 0 {
		t.Fatal("应至少创建一次")
	}
}

func TestMultiple(t *testing.T) {
	p := New(func() *box { return &box{v: -1} }, nil)
	items := make([]*box, 5)
	for i := range items {
		items[i] = p.Get()
	}
	for _, it := range items {
		p.Put(it)
	}
	count := 0
	for i := 0; i < 5; i++ {
		b := p.Get()
		if b.v == -1 {
			count++
		}
		p.Put(b)
	}
	if count == 0 {
		t.Fatal("pool reuse")
	}
}
