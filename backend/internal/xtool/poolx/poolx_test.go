package poolx

import (
	"testing"
)

type Buf struct {
	Data []byte
}

func newBuf() any {
	return &Buf{Data: make([]byte, 0, 64)}
}

func resetBuf(v any) {
	b := v.(*Buf)
	b.Data = b.Data[:0]
}

func TestGetPut(t *testing.T) {
	p := New(2, newBuf, resetBuf)
	b1 := p.Get().(*Buf)
	p.Put(b1)
	b2 := p.Get().(*Buf)
	if b1 != b2 {
		t.Fatal("应复用")
	}
}

func TestCapacity(t *testing.T) {
	p := New(1, newBuf, resetBuf)
	p.Put(&Buf{})
	if err := p.Put(&Buf{}); err != nil {
		t.Fatal("put")
	}
	if p.Len() > 1 {
		t.Fatal("超容量")
	}
}

func TestPut_Nil(t *testing.T) {
	p := New(1, newBuf, resetBuf)
	if err := p.Put(nil); err == nil {
		t.Fatal("应报错")
	}
}

func TestUse(t *testing.T) {
	p := New(1, newBuf, resetBuf)
	p.Use(func(v any) {
		b := v.(*Buf)
		b.Data = append(b.Data, 1, 2)
	})
	if p.Len() != 1 {
		t.Fatal("Use 后应放回")
	}
}

func TestNew_Factory(t *testing.T) {
	calls := 0
	p := New(0, func() any { calls++; return &Buf{} }, nil)
	if p.Capacity() <= 0 {
		t.Fatal("cap")
	}
	p.Get()
	if calls == 0 {
		t.Fatal("factory")
	}
}
