package poolx

import "testing"

type buf struct {
	data []byte
}

func TestGetPut(t *testing.T) {
	p := New(func() *buf { return &buf{data: make([]byte, 0, 16)} }, func(b *buf) {
		b.data = b.data[:0]
	})
	b := p.Get()
	b.data = append(b.data, 1, 2, 3)
	p.Put(b)
	b2 := p.Get()
	if len(b2.data) != 0 {
		t.Fatal("reset")
	}
}

func TestUse(t *testing.T) {
	p := New(func() []byte { return make([]byte, 0) }, func(b []byte) { _ = b })
	p.Use(func(b []byte) { b = append(b, 1) })
}

func TestGet_NoReset(t *testing.T) {
	p := New(func() int { return 7 }, nil)
	if v := p.Get(); v != 7 {
		t.Fatal("v")
	}
}
