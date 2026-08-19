package randomx

import "testing"

func TestBytes(t *testing.T) {
	b, err := Bytes(16)
	if err != nil || len(b) != 16 {
		t.Fatal("bytes")
	}
}

func TestHex(t *testing.T) {
	s, err := Hex(8)
	if err != nil || len(s) != 16 {
		t.Fatal("hex", s)
	}
}

func TestInt63(t *testing.T) {
	v, err := Int63(100)
	if err != nil || v < 0 || v >= 100 {
		t.Fatal("int63")
	}
}

func TestInt63_Zero(t *testing.T) {
	v, err := Int63(0)
	if err != nil || v != 0 {
		t.Fatal("zero")
	}
}

func TestString(t *testing.T) {
	s, err := String(10)
	if err != nil || len(s) != 10 {
		t.Fatal("str", s)
	}
}

func TestRandomness(t *testing.T) {
	a, _ := Bytes(8)
	b, _ := Bytes(8)
	if string(a) == string(b) {
		t.Fatal("应不同")
	}
}
