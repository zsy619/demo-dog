package random

import (
	"testing"
)

func TestBytes(t *testing.T) {
	b, err := Bytes(16)
	if err != nil || len(b) != 16 {
		t.Fatal("bytes")
	}
}

func TestHex(t *testing.T) {
	s, _ := Hex(8)
	if len(s) != 16 {
		t.Fatal("hex")
	}
}

func TestBase64(t *testing.T) {
	s, _ := Base64(8)
	if s == "" {
		t.Fatal("base64")
	}
}

func TestInt(t *testing.T) {
	for i := 0; i < 100; i++ {
		n, _ := Int(10)
		if n < 0 || n >= 10 {
			t.Fatal("int")
		}
	}
}

func TestChoice(t *testing.T) {
	v, err := Choice([]int{1, 2, 3})
	if err != nil || v == 0 {
		t.Fatal("choice")
	}
}

func TestChoiceEmpty(t *testing.T) {
	if _, err := Choice([]int{}); err == nil {
		t.Fatal("应报错")
	}
}

func TestInt_BadMax(t *testing.T) {
	if _, err := Int(0); err == nil {
		t.Fatal("应报错")
	}
}

func TestToken(t *testing.T) {
	tk, _ := Token()
	if len(tk) != 64 {
		t.Fatal("token")
	}
}

func TestDistinctness(t *testing.T) {
	a, _ := Hex(8)
	b, _ := Hex(8)
	if a == b {
		t.Fatal("应不同")
	}
}
