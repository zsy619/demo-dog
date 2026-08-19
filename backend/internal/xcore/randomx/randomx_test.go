package randomx

import "testing"

func TestString(t *testing.T) {
	s := String(10)
	if len(s) != 10 {
		t.Fatal("len", s)
	}
}

func TestHex(t *testing.T) {
	h := Hex(8)
	if len(h) != 16 {
		t.Fatal("hex", h)
	}
}

func TestBytes(t *testing.T) {
	b := Bytes(4)
	if len(b) != 4 {
		t.Fatal("bytes")
	}
}

func TestInt(t *testing.T) {
	for i := 0; i < 100; i++ {
		v := Int(10)
		if v < 0 || v >= 10 {
			t.Fatal("int", v)
		}
	}
}

func TestIntZero(t *testing.T) {
	if Int(0) != 0 {
		t.Fatal("zero")
	}
}
